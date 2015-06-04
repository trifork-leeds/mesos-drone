# mesos-drone
Initial work on an Apache Mesos framework for Drone Ci

# Notes
## Building scheduler/executor example
```
cd to go/src/github.com/mesos-drone/
go build -tags=drone-sched -o drone-scheduler scheduler/DroneScheduler.go
go build -tags=drone-exec -o drone-executor executor/executor.go
```
## Running a scheduler on a master/slave
```
sudo ./drone-scheduler -address="INTERNAL_IP_OF_CURRENT_MACHINE" -master="INTERNAL_MASTER_IP:5050" -v=1 -executor="ABSOLUTE_PATH_TO_EXECUTOR" -logtostderr=true -droneip="-addr=MASTER_IP:8000"
```
