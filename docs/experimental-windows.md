# Running k0s worker nodes in Windows

## Experimental status

Windows support feature is under active development and MUST BE considered as experemential.

## Requirements

The cluster must have at least one worker node and control plane running on Linux. Windows can be used for running additional worker nodes.

## Build

`make clean k0s.exe`

This should create k0s.exe with staged kubelet.exe and kube-proxy.exe

## Description
the k0s.exe supervises kubelet.exe and kube-proxy.exe
During the first run calico install script created as `C:\bootstrap.ps1`

The bootstrap script downloads the calico binaries, builds pause container and set ups vSwitch settings.
 

## Running

It is expected to have docker EE installed on the windows node (we need it during the initial calico set up)

```
C:\>k0s.exe worker --cri-socket=docker:tcp://127.0.0.1:2375 --cidr-range=<cidr_range> --cluster-dns=<clusterdns> --api-server=<k0s api> <token>
```

Cluster control plane must be inited with proper config (see section below)

## Configuration

### Strict-affinity

To run windows node we need to have strict affinity enabled.

There is a configuration field `spec.network.calico.withWindowsNodes`, equals false by default.
If set to the true, the additional calico related manifest `/var/lib/k0s/manifests/calico/calico-IPAMConfig-ipamconfig.yaml` would be created with the following values

```
---
apiVersion: crd.projectcalico.org/v1
kind: IPAMConfig
metadata:
  name: default
spec:
  strictAffinity: true
```
Another way is to use calicoctl manually:
```
calicoctl ipam configure --strictaffinity=true
```

### Network connectivity in AWS
The network interface attached to your EC2 instance MUST have disabled “Change Source/Dest. Check” option.
In AWS console option can be found on the Actions menu for a selected network interface.

### Hacks

We need to figure out proper way to pass cluster settings from controller plane to worker.

While we don't have it, there are CLI arguments:
- cidr-range
- cluster-dns
- api-server 


## Some useful commands

Run pod with cmd.exe shell
```
kubectl run win --image=hello-world:nanoserver --command=true -i --attach=true -- cmd.exe
```

Manifest for pod with IIS web-server
```
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