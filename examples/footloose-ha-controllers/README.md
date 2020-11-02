# k0s HA controllers with footloose

> This example is targeted for development use

## Setup

* Install [footloose](https://github.com/weaveworks/footloose)
* Start a cluster: `make create-cluster`
* Now you can exec into running nodes, for example: `footloose ssh root@controller0`
* Within a node there is a config file (`/etc/k0s/config.yaml`) and systemd service (`k0s`) available for easier testing.


## Teardown

* Teardown a cluster: `make delete-cluster`
