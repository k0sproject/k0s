# Run k0s worker nodes in Windows

**IMPORTANT**: Windows support for k0s is under active development and **must be** considered experimental.

## Prerequisites

The cluster must be running at least one worker node and control plane on Linux. You ca use Windows to run additional worker nodes. 

## Build k0s.exe

Invoke the `make clean k0s.exe` command to create k0s.exe with staged kubelet.exe and kube-proxy.exe.

**Note**: The k0s.exe supervises kubelet.exe and kube-proxy.exe.

During the first run, the calico install script is creaated as `C:\bootstrap.ps1`. This bootstrap script downloads the calico binaries, builds pause container and set ups vSwitch settings. 

## Run k0s

Install Mirantis Container Runtime on the Windows node(s), as it is required for the initial Calico set up). 

```
C:\>k0s.exe worker --cri-socket=docker:tcp://127.0.0.1:2375 --cidr-range=<cidr_range> --cluster-dns=<clusterdns> --api-server=<k0s api> <token>
```

You must initiate the Cluster control with the correct config.

## Configuration

### Strict-affinity

You must enable strict affinity to run the windows node.

If the `spec.network.calico.withWindowsNodes` field is set to `true` (it is set to `false` by default) the additional calico related manifest `/var/lib/k0s/manifests/calico/calico-IPAMConfig-ipamconfig.yaml` is created with the following values: 

```yaml
---
apiVersion: crd.projectcalico.org/v1
kind: IPAMConfig
metadata:
  name: default
spec:
  strictAffinity: true
```
Alternately, you can manually execute calicoctl:
```
calicoctl ipam configure --strictaffinity=true
```

### Network connectivity in AWS

Disable the `Change Source/Dest. Check` option for the network interface attached to your EC2 instance. In AWS, the console option for the network interface is in the **Actions** menu. 

### Hacks

k0s offers the following CLI arguments in lieu of a formal means for passing cluster settings from controller plane to worker: 

- cidr-range
- cluster-dns
- api-server 


## Useful commands

### Run pod with cmd.exe shell

```
kubectl run win --image=hello-world:nanoserver --command=true -i --attach=true -- cmd.exe
```

### Manifest for pod with IIS web-server

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: iis
spec:
  containers:
  - name: iis
    image: mcr.microsoft.com/windows/servercore/iis
    imagePullPolicy: IfNotPresent
```