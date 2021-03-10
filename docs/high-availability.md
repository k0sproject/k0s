## Control Plane High Availability

The following pre-requisites are required in order to configure an HA control plane:
 
### Requirements
##### Load Balancer
A load balancer with a single external address should be configured as the IP gateway for the controllers.
The load balancer should allow traffic to each controller on the following ports:

- 6443
- 8132
- 8133
- 9443

##### Cluster configuration
On each controller node, a k0s.yaml configuration file should be configured.
The following options need to match on each node, otherwise the control plane components will end up in very unknown states:

- `network`
- `storage`: Needless to say, one cannot create a clustered control plane with each node only storing data locally on SQLite.
- `externalAddress`

[Full configuration file refrence](configuration.md)