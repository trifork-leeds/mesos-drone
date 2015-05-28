# mesos-drone
Initial work on an Apache Mesos framework for Drone Ci

# Notes
## Running a scheduler on a master/slave
```
./scheduler -logtostderr=true -task-count="1" -v=1 -executor="PATH_TO_EXECUTOR" -master="MASTER_IP:5050"  -address="PUBLIC_IP_OF_CURRENT_MACHINE"
```
