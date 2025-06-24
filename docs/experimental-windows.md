# Run k0s worker nodes in Windows

**IMPORTANT**: Windows support for k0s is under active development and **must be** considered experimental.

## Prerequisites

The cluster must be running at least one worker node and control plane on Linux. You can use Windows to run additional worker nodes.

## Run k0s

**Note**: k0s supervises `kubelet.exe` and `kube-proxy.exe`.

During the first run, the Calico install script is created as `C:\bootstrap.ps1`. This bootstrap script downloads the Calico binaries, builds pause container and sets up vSwitch settings.

Install Mirantis Container Runtime on the Windows node(s), as it is required for the initial Calico set up.

```shell
k0s worker --cri-socket=remote:npipe:////./pipe/containerd-containerd <token>
```

You must initiate the cluster control with the correct config.

## Configuration

### Strict-affinity

You must enable strict affinity to run the windows node.

If the `spec.network.Calico.withWindowsNodes` field is set to `true` (it is set to `false` by default) the additional Calico related manifest `/var/lib/k0s/manifests/calico/calico-IPAMConfig-ipamconfig.yaml` is created with the following values:

```yaml
---
apiVersion: crd.projectcalico.org/v1
kind: IPAMConfig
metadata:
  name: default
spec:
  strictAffinity: true
```

Alternately, you can manually execute `calicoctl`:

```shell
calicoctl ipam configure --strictaffinity=true
```

### Network connectivity in AWS

Disable the `Change Source/Dest. Check` option for the network interface attached to your EC2 instance. In AWS, the console option for the network interface is in the **Actions** menu.

## Useful commands

### Run pod with cmd.exe shell

```shell
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
