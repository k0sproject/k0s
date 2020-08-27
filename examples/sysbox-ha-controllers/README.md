# MKE HA controllers with sysbox

> This example is targeted for development use

## Setup

* Install & configure [sysbox](https://github.com/nestybox/sysbox)
* Start a cluster: `make cluster`
* Now you can exec into running nodes, for example: `docker-compose exec controller1 bash`
* Within a node there is a built mke binary available for testing stuff.


## Teardown

* Teardown a cluster: `make teardown`
