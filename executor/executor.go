
package main

import (
	"flag"
	exec "github.com/mesos/mesos-go/executor"
	mesos "github.com/mesos/mesos-go/mesosproto"
	log "github.com/golang/glog"
	exe "os/exec"
	"os"
)

type exampleExecutor struct {
	tasksLaunched int
}

func newExampleExecutor() *exampleExecutor {
	return &exampleExecutor{tasksLaunched: 0}
}

func (exec *exampleExecutor) Registered(driver exec.ExecutorDriver, execInfo *mesos.ExecutorInfo, fwinfo *mesos.FrameworkInfo, slaveInfo *mesos.SlaveInfo) {
	log.Info("Registered Executor on slave ", slaveInfo.GetHostname())
}

func (exec *exampleExecutor) Reregistered(driver exec.ExecutorDriver, slaveInfo *mesos.SlaveInfo) {
	log.Info("Re-registered Executor on slave ", slaveInfo.GetHostname())
}

func (exec *exampleExecutor) Disconnected(exec.ExecutorDriver) {
	log.Info("Executor disconnected.")
}

func (exec *exampleExecutor) LaunchTask(driver exec.ExecutorDriver, taskInfo *mesos.TaskInfo) {
	log.Info("Launching task", taskInfo.GetName(), "with command", taskInfo.Command.GetValue())

	runStatus := &mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesos.TaskState_TASK_RUNNING.Enum(),
	}
	_, err := driver.SendStatusUpdate(runStatus)
	if err != nil {
		log.Error("Got error", err)
	}

	exec.tasksLaunched++
	log.Info("Total tasks launched ", exec.tasksLaunched)

	log.Info("Executing drone-agent")
	droneCmd := exe.Command("drone-agent","-addr=http://localhost:8000","-token=1")
	droneCmd.Stdout = os.Stdout
	droneCmd.Stderr = os.Stderr
	err = droneCmd.Run()
	if err != nil {
		panic(err)
	}
	log.Info("Completed drone-agent")

	// finish task
	log.Info("Finishing task", taskInfo.GetName())
	finStatus := &mesos.TaskStatus{
		TaskId: taskInfo.GetTaskId(),
		State:  mesos.TaskState_TASK_FINISHED.Enum(),
	}
	_, err = driver.SendStatusUpdate(finStatus)
	if err != nil {
		log.Error("Got error", err)
	}
	log.Info("Task finished", taskInfo.GetName())
}

func (exec *exampleExecutor) KillTask(exec.ExecutorDriver, *mesos.TaskID) {
	log.Info("Kill task")
}

func (exec *exampleExecutor) FrameworkMessage(driver exec.ExecutorDriver, msg string) {
	log.Info("Got framework message: ", msg)
}

func (exec *exampleExecutor) Shutdown(exec.ExecutorDriver) {
	log.Info("Shutting down the executor")
}

func (exec *exampleExecutor) Error(driver exec.ExecutorDriver, err string) {
	log.Error("Got error message:", err)
}

// -------------------------- func inits () ----------------- //
func init() {
	flag.Parse()
}

func main() {
	log.Info("Starting Example Executor (Go)")

	dconfig := exec.DriverConfig{
		Executor: newExampleExecutor(),
	}
	driver, err := exec.NewMesosExecutorDriver(dconfig)

	if err != nil {
		log.Warning("Unable to create a ExecutorDriver ", err.Error())
	}

	_, err = driver.Start()
	if err != nil {
		log.Error("Got error:", err)
		return
	}
	log.Info("Executor process has started and running.")
	driver.Join()
}
