# Control Plane High Availability

You can create high availability for the control plane by distributing the control plane across multiple nodes and installing a load balancer on top. Etcd can be colocated with the controller nodes (default in k0s) to achieve highly available datastore at the same time.

![k0s high availability](img/k0s_high_availability.png)

**Note:** In this context even 2 node controlplane is considered HA even though it's not really HA from etcd point of view. The same requirement for [LB](#load-balancer) still applies.

## Network considerations

You should plan to allocate the control plane nodes into different zones. This will avoid failures in case one zone fails.

For etcd high availability it's recommended to configure 3 or 5 controller nodes. For more information, refer to the [etcd documentation](https://etcd.io/docs/latest/faq/#why-an-odd-number-of-cluster-members).

## Load Balancer

Control plane high availability requires a tcp load balancer, which acts as a single point of contact to access the controllers. The load balancer needs to allow and route traffic to each controller through the following ports:

- 6443 (for Kubernetes API)
- 8132 (for Konnectivity)
- 9443 (for controller join API)

The load balancer can be implemented in many different ways and k0s doesn't have any additional requirements. You can use for example HAProxy, NGINX or your cloud provider's load balancer.

### Example configuration: HAProxy

Add the following lines to the end of the haproxy.cfg:

```txt
frontend kubeAPI
    bind :6443
    mode tcp
    default_backend kubeAPI_backend
frontend konnectivity
    bind :8132
    mode tcp
    default_backend konnectivity_backend
frontend controllerJoinAPI
    bind :9443
    mode tcp
    default_backend controllerJoinAPI_backend

backend kubeAPI_backend
    mode tcp
    server k0s-controller1 <ip-address1>:6443 check check-ssl verify none
    server k0s-controller2 <ip-address2>:6443 check check-ssl verify none
    server k0s-controller3 <ip-address3>:6443 check check-ssl verify none
backend konnectivity_backend
    mode tcp
    server k0s-controller1 <ip-address1>:8132 check check-ssl verify none
    server k0s-controller2 <ip-address2>:8132 check check-ssl verify none
    server k0s-controller3 <ip-address3>:8132 check check-ssl verify none
backend controllerJoinAPI_backend
    mode tcp
    server k0s-controller1 <ip-address1>:9443 check check-ssl verify none
    server k0s-controller2 <ip-address2>:9443 check check-ssl verify none
    server k0s-controller3 <ip-address3>:9443 check check-ssl verify none

listen stats
   bind *:9000
   mode http
   stats enable
   stats uri /
```

The last block "listen stats" is optional, but can be helpful. It enables HAProxy statistics with a separate dashboard to monitor for example the health of each backend server. You can access it using a web browser:

```txt
http://<ip-addr>:9000
```

Restart HAProxy to apply the configuration changes.

### Example configuration: Citrix VPX Load Balancer

**Note: This is extremely expermintial; so use at your own risk.**

The config below shows how you can input this into the CLI on your Citrix VPX load balancer. You **must** replace **[IP ADDRESS]** with your network IPv4 address settings.

#### Controller Entries
```txt
add server srv_kube_controller_01 [IP ADDRESS]
add server srv_kube_controller_02 [IP ADDRESS]
```
Add as many k0s controllers you plan on having.

#### Service Groups
```txt
add serviceGroup sg_kube_controllers ANY -maxClient 0 -maxReq 0 -cip DISABLED -usip NO -useproxyport NO -cltTimeout 120 -svrTimeout 120 -CKA NO -TCPB NO -CMP NO -appflowLog DISABLED
```

#### LB vServer
```txt
add lb vserver lb_control_kubernetes ANY [IP ADDRESS] * -persistenceType NONE -timeout 15 -Listenpolicy "CLIENT.TCP.DSTPORT.EQ(6443)||Client.TCP.DSTPORT.EQ(8132)||Client.TCP.DSTPORT.EQ(9443)" -Listenpriority 1 -cltTimeout 120 -appflowLog DISABLED
bind lb vserver lb_control_kubernetes sg_kube_controllers
```

#### LB Monitor Settings
```txt
add lb monitor tcp_kube_6443 TCP -LRTM DISABLED -destPort 6443
add lb monitor tcp_kube_8132 TCP -LRTM DISABLED -destPort 8132
add lb monitor tcp_kube_9443 TCP -LRTM DISABLED -destPort 9443
```

#### Service Group Members
```txt
bind serviceGroup sg_kube_controllers srv_kube_controller_02 *
bind serviceGroup sg_kube_controllers srv_kube_controller_01 *
```
Be sure to include all your Controller entries here.

#### Monitor Bindings
```txt
bind serviceGroup sg_kube_controllers -monitorName tcp_kube_6443
bind serviceGroup sg_kube_controllers -monitorName tcp_kube_8132
bind serviceGroup sg_kube_controllers -monitorName tcp_kube_9443
```

## k0s configuration

The load balancer address must be configured to k0s either by using `k0s.yaml` or by using k0sctl to automatically deploy all controllers with the same configuration:

### Configuration using k0s.yaml (for each controller)

Note to update your load balancer's public ip address into two places.

```yaml
spec:
  api:
    externalAddress: <load balancer public ip address>
    sans:
    - <load balancer public ip address>
```

### Configuration using k0sctl.yaml (for k0sctl)

Add the following lines to the end of the k0sctl.yaml. Note to update your load balancer's public ip address into two places.

```yaml
  k0s:
    config:
      spec:
        api:
          externalAddress: <load balancer public ip address>
          sans:
          - <load balancer public ip address>
```

For greater detail about k0s configuration, refer to the [Full configuration file reference](configuration.md).
