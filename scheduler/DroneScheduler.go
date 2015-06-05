// +build example-sched

/**
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements.  See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership.  The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License.  You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"os/exec"
	"github.com/gogo/protobuf/proto"
	log "github.com/golang/glog"
	"github.com/mesos/mesos-go/auth"
	"github.com/mesos/mesos-go/auth/sasl"
	"github.com/mesos/mesos-go/auth/sasl/mech"
	mesos "github.com/mesos/mesos-go/mesosproto"
	util "github.com/mesos/mesos-go/mesosutil"
	sched "github.com/mesos/mesos-go/scheduler"
	"golang.org/x/net/context"
	"github.com/drone/drone/pkg/queue"
	"encoding/json"
	"bytes"
)

const (
	CPUS_PER_TASK       = 1
	MEM_PER_TASK        = 128
	defaultArtifactPort = 12345
)

var (
	//address      = flag.String("address", "127.0.0.1", "Binding address for artifact server")
	artifactPort = flag.Int("artifactPort", defaultArtifactPort, "Binding port for artifact server")
	authProvider = flag.String("mesos_authentication_provider", sasl.ProviderName,
		fmt.Sprintf("Authentication provider to use, default is SASL that supports mechanisms: %+v", mech.ListSupported()))
	//master              = flag.String("master", "127.0.0.1:5050", "Master address <ip:port>")
	executorPath        = flag.String("executor", "./example_executor", "Path to test executor")
	mesosAuthPrincipal  = flag.String("mesos_authentication_principal", "", "Mesos authentication principal.")
	mesosAuthSecretFile = flag.String("mesos_authentication_secret_file", "", "Mesos authentication secret file.")
	droneServerIP		= flag.String("droneip","127.0.0.1:80","IP address of the drone server")

	//used to keep a track of launched items in the queue
	launchedTasks = make(map[string](interface{}))
	master string
	address string
)

type ExampleScheduler struct {
	executor      *mesos.ExecutorInfo
	tasksLaunched int
	tasksFinished int
	totalTasks    int
}

func newExampleScheduler(exec *mesos.ExecutorInfo) *ExampleScheduler {

	return &ExampleScheduler{
		executor:      exec,
		tasksLaunched: 0,
		tasksFinished: 0,
	}
}

func (sched *ExampleScheduler) Registered(driver sched.SchedulerDriver, frameworkId *mesos.FrameworkID, masterInfo *mesos.MasterInfo) {
	log.Infoln("Framework Registered with Master ", masterInfo)
}

func (sched *ExampleScheduler) Reregistered(driver sched.SchedulerDriver, masterInfo *mesos.MasterInfo) {
	log.Infoln("Framework Re-Registered with Master ", masterInfo)
}

func (sched *ExampleScheduler) Disconnected(sched.SchedulerDriver) {}

func (sched *ExampleScheduler) ResourceOffers(driver sched.SchedulerDriver, offers []*mesos.Offer) {

	for _, offer := range offers {
		cpuResources := util.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
			return res.GetName() == "cpus"
		})
		cpus := 0.0
		for _, res := range cpuResources {
			cpus += res.GetScalar().GetValue()
		}

		memResources := util.FilterResources(offer.Resources, func(res *mesos.Resource) bool {
			return res.GetName() == "mem"
		})
		mems := 0.0
		for _, res := range memResources {
			mems += res.GetScalar().GetValue()
		}

		log.Infoln("Received Offer <", offer.Id.GetValue(), "> with cpus=", cpus, " mem=", mems)

		remainingCpus := cpus
		remainingMems := mems
		var tasks []*mesos.TaskInfo
		queueItems := getQueue(sched)
		if (len(queueItems) == 0){
			fmt.Println("Queue is empty!")
		}

		// start a task of each item in the queue. before starting, check
		// if it has not already been launched.
		for i, item := range queueItems {
			if val, ok := launchedTasks[item.Commit.Sha]; ok{
				fmt.Println("item exists: ", val)
				continue
			}

			fmt.Println("Adding item: ", item.Commit.Sha)
			launchedTasks[item.Commit.Sha] = item.Commit.Sha

			if(CPUS_PER_TASK <= remainingCpus && MEM_PER_TASK <= remainingMems){

				sched.tasksLaunched += i

				taskId := &mesos.TaskID{
					Value: proto.String(strconv.Itoa(sched.tasksLaunched)),
				}

				stringArray := []string{"-addr=" + *droneServerIP, "-token=1"}
				dataString := strings.Join(stringArray, " ")

				task := &mesos.TaskInfo{
					Name:     proto.String("go-task-" + taskId.GetValue()),
					TaskId:   taskId,
					SlaveId:  offer.SlaveId,
					Executor: sched.executor,
					Data: []byte(dataString),
					Resources: []*mesos.Resource{
						util.NewScalarResource("cpus", CPUS_PER_TASK),
						util.NewScalarResource("mem", MEM_PER_TASK),
					},
				}

				log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), offer.Id.GetValue())

				tasks = append(tasks, task)
				remainingCpus -= CPUS_PER_TASK
				remainingMems -= MEM_PER_TASK
			}

		}

//		for sched.tasksLaunched < sched.totalTasks &&
//		CPUS_PER_TASK <= remainingCpus &&
//		MEM_PER_TASK <= remainingMems {
//
//			sched.tasksLaunched++
//
//			taskId := &mesos.TaskID{
//				Value: proto.String(strconv.Itoa(sched.tasksLaunched)),
//			}
//
//			stringArray := []string{*droneServerIP, "-token=1"}
//			dataString := strings.Join(stringArray, " ")
//
//			task := &mesos.TaskInfo{
//				Name:     proto.String("go-task-" + taskId.GetValue()),
//				TaskId:   taskId,
//				SlaveId:  offer.SlaveId,
//				Executor: sched.executor,
//				Data: []byte(dataString),
//				Resources: []*mesos.Resource{
//					util.NewScalarResource("cpus", CPUS_PER_TASK),
//					util.NewScalarResource("mem", MEM_PER_TASK),
//				},
//			}
//
//			log.Infof("Prepared task: %s with offer %s for launch\n", task.GetName(), offer.Id.GetValue())
//
//			tasks = append(tasks, task)
//			remainingCpus -= CPUS_PER_TASK
//			remainingMems -= MEM_PER_TASK
//		}
		log.Infoln("Launching ", len(tasks), "tasks for offer", offer.Id.GetValue())
		driver.LaunchTasks([]*mesos.OfferID{offer.Id}, tasks, &mesos.Filters{RefuseSeconds: proto.Float64(1)})
	}
}

func (sched *ExampleScheduler) StatusUpdate(driver sched.SchedulerDriver, status *mesos.TaskStatus) {
	log.Infoln("Status update: task", status.TaskId.GetValue(), " is in state ", status.State.Enum().String())
	if status.GetState() == mesos.TaskState_TASK_FINISHED {
		sched.tasksFinished++
	}

	if sched.tasksFinished >= sched.totalTasks {
		log.Infoln("Total tasks completed, stopping framework.")
		driver.Stop(false)
	}

	if status.GetState() == mesos.TaskState_TASK_LOST ||
	status.GetState() == mesos.TaskState_TASK_KILLED ||
	status.GetState() == mesos.TaskState_TASK_FAILED {
		log.Infoln(
			"Aborting because task", status.TaskId.GetValue(),
			"is in unexpected state", status.State.String(),
			"with message", status.GetMessage(),
		)
		driver.Abort()
	}
}

func (sched *ExampleScheduler) OfferRescinded(sched.SchedulerDriver, *mesos.OfferID) {}

func (sched *ExampleScheduler) FrameworkMessage(driver sched.SchedulerDriver,exec *mesos.ExecutorID,slave *mesos.SlaveID,msg string) {
}
func (sched *ExampleScheduler) SlaveLost(sched.SchedulerDriver, *mesos.SlaveID) {}
func (sched *ExampleScheduler) ExecutorLost(sched.SchedulerDriver, *mesos.ExecutorID, *mesos.SlaveID, int) {
}

func (sched *ExampleScheduler) Error(driver sched.SchedulerDriver, err string) {
	log.Infoln("Scheduler received error:", err)
}

// ----------------------- func init() ------------------------- //

func init() {
	flag.Parse()
	log.Infoln("Initializing the Example Scheduler...")
	master = strings.TrimSpace(getMasterIP())
	address = strings.Split(master,":")[0]
}

// returns (downloadURI, basename(path))
func serveExecutorArtifact(path string) (*string, string) {
	serveFile := func(pattern string, filename string) {
		http.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, filename)
		})
	}

	// Create base path (http://foobar:5000/<base>)
	pathSplit := strings.Split(path, "/")
	var base string
	if len(pathSplit) > 0 {
		base = pathSplit[len(pathSplit)-1]
	} else {
		base = path
	}
	serveFile("/"+base, path)

	hostURI := fmt.Sprintf("http://%s:%d/%s", address, *artifactPort, base)
	log.V(2).Infof("Hosting artifact '%s' at '%s'", path, hostURI)

	return &hostURI, base
}

func prepareExecutorInfo() *mesos.ExecutorInfo {
	executorUris := []*mesos.CommandInfo_URI{}
	uri, executorCmd := serveExecutorArtifact(*executorPath)
	executorUris = append(executorUris, &mesos.CommandInfo_URI{Value: uri, Executable: proto.Bool(true)})

	executorCommand := fmt.Sprintf("./%s", executorCmd)

	go http.ListenAndServe(fmt.Sprintf("%s:%d", address, *artifactPort), nil)
	log.V(2).Info("Serving executor artifacts...")

	// Create mesos scheduler driver.
	return &mesos.ExecutorInfo{
		ExecutorId: util.NewExecutorID("default"),
		Name:       proto.String("Test Executor (Go)"),
		Source:     proto.String("go_test"),
		Command: &mesos.CommandInfo{
			Value: proto.String(executorCommand),
			Uris:  executorUris,
		},
	}
}

func parseIP(address string) net.IP {
	addr, err := net.LookupIP(address)
	if err != nil {
		log.Fatal(err)
	}
	if len(addr) < 1 {
		log.Fatalf("failed to parse IP from address '%v'", address)
	}
	return addr[0]
}

/**
	Queries drone server to get queue of drone tasks.
	Counts the number of pending commits and pass them to the
	given scheduler as the total number  of tasks to be executed.
	@param Scheduler the scheduler to pass number of pending commits
	@return an array of queue work
 */
func getQueue(sched *ExampleScheduler)([]*queue.Work) {

	uri := "http://" + *droneServerIP + "/api/queue/get?token=1"
	response, err := http.Get(uri)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()
	var resp []*queue.Work
	body, _ := ioutil.ReadAll(response.Body)
	err = json.Unmarshal(body, &resp)
	if err != nil {
		panic(err)
	}

	for _, item := range resp {
		if (item.Commit.State == "pending") {
			sched.totalTasks++
		}

	}

	return resp
}

/**
	Uses mesos cli to query zookeeper to resolve master ip.
	@return string master ip including port number
 */
func getMasterIP()string{

	cmd := exec.Command("/bin/bash", "-c","mesos-resolve $(cat /etc/mesos/zk) 2>/dev/null")
	cmdOutput := &bytes.Buffer{}
	cmd.Stdout = cmdOutput
	err := cmd.Run()
	if err != nil{
		panic(err.Error())
	}

	if(len(string(cmdOutput.Bytes())) < 1){
		log.Fatal("Couldn't solve master ip.")
	}
	return string(cmdOutput.Bytes())
}

/**
	Starts Drone server on a background process.
 */
func startDrone(){

	cmd := exec.Command("/bin/sh", "-c","drone --config=config.toml &")
	err := cmd.Run()
	if err != nil {
		panic(err)
	}
}

// ----------------------- func main() ------------------------- //

func main() {

	startDrone()

	// build command executor
	exec := prepareExecutorInfo()

	// the framework
	fwinfo := &mesos.FrameworkInfo{
		User: proto.String(""), // Mesos-go will fill in user.
		Name: proto.String("Drone Framework"),
	}

	cred := (*mesos.Credential)(nil)
	if *mesosAuthPrincipal != "" {
		fwinfo.Principal = proto.String(*mesosAuthPrincipal)
		secret, err := ioutil.ReadFile(*mesosAuthSecretFile)
		if err != nil {
			log.Fatal(err)
		}
		cred = &mesos.Credential{
			Principal: proto.String(*mesosAuthPrincipal),
			Secret:    secret,
		}
	}

	fmt.Println("Address: ",address)
	fmt.Println("Master: ",master)
	bindingAddress := parseIP(address)
	fmt.Println("bindingAddress: ",bindingAddress)
	config := sched.DriverConfig{
		Scheduler:      newExampleScheduler(exec),
		Framework:      fwinfo,
		Master:         master,
		Credential:     cred,
		BindingAddress: bindingAddress,
		WithAuthContext: func(ctx context.Context) context.Context {
			ctx = auth.WithLoginProvider(ctx, *authProvider)
			ctx = sasl.WithBindingAddress(ctx, bindingAddress)
			return ctx
		},
	}

	driver, err := sched.NewMesosSchedulerDriver(config)

	if err != nil {
		log.Errorln("Unable to create a SchedulerDriver ", err.Error())
	}

	if stat, err := driver.Run(); err != nil {
		log.Infof("Framework stopped with status %s and error: %s\n", stat.String(), err.Error())
	}

}
