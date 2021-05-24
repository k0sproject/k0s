# Control Plane High Availability

The configuration of a high availability control plane for k0s requires the deployment of both a load balancer and a cluster configuration file.

## Load Balancer

Configure a load balancer with a single external address as the IP gateway for the controllers. Set the load balancer to allow traffic to each controller through the following ports:

- 6443
- 8132
- 8133
- 9443

## Cluster configuration

Configure a `k0s.yaml` configuration file for each controller node with the following options:

- `network`
- `storage`
- `externalAddress`

For greater detail, refer to the [Full configuration file reference](configuration.md).