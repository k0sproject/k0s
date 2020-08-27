# MKE HA controllers with footloose

> This example is targeted for development use

## Setup

* Install [footloose](https://github.com/weaveworks/footloose)
* Start a cluster: `make create-cluster`
* Now you can exec into running nodes, for example: `footloose ssh root@controller0`
* Within a node there is a config file (`/etc/mke/config.yaml`) and systemd service (`mke`) available for easier testing.


## Teardown

* Teardown a cluster: `make delete-cluster`
