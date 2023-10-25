# k0s HA controllers with bootloose

> This example is targeted for development use

## Setup

* Install [bootloose](https://github.com/k0sproject/bootloose)
* Start a cluster: `make create-cluster`
* Now you can exec into running nodes, for example: `bootloose ssh root@controller0`
* Within a node there is a config file (`/etc/k0s/config.yaml`) and systemd service (`k0s`) available for easier testing.

## Teardown

* Teardown a cluster: `make delete-cluster`
