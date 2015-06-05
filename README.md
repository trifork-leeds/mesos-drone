# mesos-drone
Initial work on an Apache Mesos framework for Drone Ci

# Notes

The drone Configuration file `config.toml` must be inside `go/src/github.com/mesos-drone/`.
## Building scheduler/executor example 
```
cd to go/src/github.com/mesos-drone/
go build -tags=drone-sched -o drone-scheduler scheduler/DroneScheduler.go
go build -tags=drone-exec -o drone-executor executor/executor.go
```
## Running a scheduler on a master/slave
```
sudo ./drone-scheduler -v=1 -executor="ABSOLUTE_PATH_TO_EXECUTOR" -logtostderr=true -droneip="DRONE_SERVER_IP:8000"
```
