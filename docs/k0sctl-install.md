# Installing with k0sctl

This tutorial is based on k0sctl tool and it's targeted for creating a multi-node cluster for remote hosts. It describes an install method, which is automatic and easily repeatable. This is recommended for production clusters and the automatic upgrade requires using this install method. The automatic upgrade process is also described in this tutorial.

k0sctl is a command-line tool for bootstrapping and managing k0s clusters. k0sctl connects to the provided hosts using SSH and gathers information about the hosts. Based on the findings it proceeds to configure the hosts, deploys k0s and connects the k0s nodes together to form a cluster.

![k0sctl deployment](img/k0sctl_deployment.png)

### Prerequisites

k0sctl can be executed on Linux, MacOS and Windows. See more details from the [k0sctl github repository](https://github.com/k0sproject/k0sctl). For hosts running k0s, see the [System Requirements](system-requirements.md).

### Installation steps

#### 1. Install k0sctl tool

k0sctl is a single binary and the installation and download instructions can be found in the [k0sctl github repository](https://github.com/k0sproject/k0sctl#installation).

#### 2. Configure the cluster

First create a k0sctl configuration file:
```sh
$ k0sctl init > k0sctl.yaml
```

A `k0sctl.yaml` file will be created in the current directory:

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
spec:
  hosts:
  - role: controller
    ssh:
      address: 10.0.0.1 # replace with the controller's IP address
      user: root
      keyPath: ~/.ssh/id_rsa
  - role: worker
    ssh:
      address: 10.0.0.2 # replace with the worker's IP address
      user: root
      keyPath: ~/.ssh/id_rsa
```

As a mandatory step, each host must be given a valid IP address (which is reachable by k0sctl) and the connection details for an SSH connection. k0sctl configuration specifications can be found in [k0sctl documentation](https://github.com/k0sproject/k0sctl#configuration-file-spec-fields).

#### 3. Deploy the cluster

Next step is to run `k0sctl apply` to perform the cluster deployment:
```sh
$ k0sctl apply --config k0sctl.yaml 

⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████

INFO k0sctl 0.0.0 Copyright 2021, Mirantis Inc.   
INFO Anonymized telemetry will be sent to Mirantis. 
INFO By continuing to use k0sctl you agree to these terms: 
INFO https://k0sproject.io/licenses/eula          
INFO ==> Running phase: Connect to hosts 
INFO [ssh] 10.0.0.1:22: connected              
INFO [ssh] 10.0.0.2:22: connected              
INFO ==> Running phase: Detect host operating systems 
INFO [ssh] 10.0.0.1:22: is running Ubuntu 20.10 
INFO [ssh] 10.0.0.2:22: is running Ubuntu 20.10 
INFO ==> Running phase: Prepare hosts    
INFO [ssh] 10.0.0.1:22: installing kubectl     
INFO ==> Running phase: Gather host facts 
INFO [ssh] 10.0.0.1:22: discovered 10.12.18.133 as private address 
INFO ==> Running phase: Validate hosts   
INFO ==> Running phase: Gather k0s facts 
INFO ==> Running phase: Download K0s on the hosts 
INFO [ssh] 10.0.0.2:22: downloading k0s 0.11.0 
INFO [ssh] 10.0.0.1:22: downloading k0s 0.11.0 
INFO ==> Running phase: Configure K0s    
WARN [ssh] 10.0.0.1:22: generating default configuration 
INFO [ssh] 10.0.0.1:22: validating configuration 
INFO [ssh] 10.0.0.1:22: configuration was changed 
INFO ==> Running phase: Initialize K0s Cluster 
INFO [ssh] 10.0.0.1:22: installing k0s controller 
INFO [ssh] 10.0.0.1:22: waiting for the k0s service to start 
INFO [ssh] 10.0.0.1:22: waiting for kubernetes api to respond 
INFO ==> Running phase: Install workers  
INFO [ssh] 10.0.0.1:22: generating token       
INFO [ssh] 10.0.0.2:22: writing join token     
INFO [ssh] 10.0.0.2:22: installing k0s worker  
INFO [ssh] 10.0.0.2:22: starting service       
INFO [ssh] 10.0.0.2:22: waiting for node to become ready 
INFO ==> Running phase: Disconnect from hosts 
INFO ==> Finished in 2m2s                
INFO k0s cluster version 0.11.0 is now installed  
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using: 
INFO      k0sctl kubeconfig              
```

And -- presto! Your k0s cluster is up and running.

#### 4. Access the cluster

To access your k0s cluster, you first need to get the kubeconfig. k0sctl does this for you like this:
```sh
$ k0sctl kubeconfig > kubeconfig
```

Then you can access your cluster for example by using kubectl or [LENS](https://k8slens.dev/).
```sh
$ kubectl get pods --kubeconfig kubeconfig -A
NAMESPACE     NAME                                       READY   STATUS    RESTARTS   AGE
kube-system   calico-kube-controllers-5f6546844f-w8x27   1/1     Running   0          3m50s
kube-system   calico-node-vd7lx                          1/1     Running   0          3m44s
kube-system   coredns-5c98d7d4d8-tmrwv                   1/1     Running   0          4m10s
kube-system   konnectivity-agent-d9xv2                   1/1     Running   0          3m31s
kube-system   kube-proxy-xp9r9                           1/1     Running   0          4m4s
kube-system   metrics-server-6fbcd86f7b-5frtn            1/1     Running   0          3m51s
```

### Upgrade a k0s cluster using k0sctl

There's no dedicated upgrade sub-command in k0sctl, the configuration file describes the desired state of the cluster and when passed to `k0sctl apply`, it will perform a discovery of the current state and do what ever is needed to bring the cluster to the desired state, by for example performing an upgrade.

#### K0sctl cluster upgrade process

The following steps will be performed during a k0sctl cluster upgrade:

1. Upgrade each controller one-by-one; As long as there’s multiple controllers configured there’s no downtime
2. Upgrade workers in batches; 10% of the worker nodes are upgraded at a time
   * Each worker is first drained allowing the workload to “move” to other nodes before the actual upgrade of the worker node components
   * The process continues after we see the upgraded nodes back in “Ready” state
   * Drain can be skipped with a --no-drain option


The desired cluster version can be configured In the k0sctl configuration by setting the value of `spec.k0s.version`:
```yaml
spec:
  k0s:
    version: 0.11.0
```

When a version has not been specified, k0sctl will check online for the latest version and default to using that.

```sh
$ k0sctl apply
...
...
INFO[0001] ==> Running phase: Upgrade controllers 
INFO[0001] [ssh] 10.0.0.23:22: starting upgrade
INFO[0001] [ssh] 10.0.0.23:22: Running with legacy service name, migrating... 
INFO[0011] [ssh] 10.0.0.23:22: waiting for the k0s service to start 
INFO[0016] ==> Running phase: Upgrade workers  
INFO[0016] Upgrading 1 workers in parallel              
INFO[0016] [ssh] 10.0.0.17:22: upgrade starting  
INFO[0027] [ssh] 10.0.0.17:22: waiting for node to become ready again 
INFO[0027] [ssh] 10.0.0.17:22: upgrade successful   
INFO[0027] ==> Running phase: Disconnect from hosts 
INFO[0027] ==> Finished in 27s                 
INFO[0027] k0s cluster version 0.11.0 is now installed 
INFO[0027] Tip: To access the cluster you can now fetch the admin kubeconfig using: 
INFO[0027]      k0sctl kubeconfig 
```

### Known limitations

* k0sctl will not perform any discovery of hosts, it only operates on the hosts listed in the provided configuration
* k0sctl can currently only add more nodes to the cluster but cannot remove existing ones

### Next Steps

- [Control plane configuration options](configuration.md) for example for networking and datastore configuration
- [Worker node configuration options](worker-node-config.md) for example for node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md) for example for load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md), a tutorial for ingress deployment
