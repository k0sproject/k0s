<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Run k0s worker nodes in Windows

**IMPORTANT**: Windows support for k0s is under active development and **must be** considered experimental.

## Prerequisites

The cluster must be running at least one worker node and control plane on Linux. You can use Windows to run additional worker nodes.

## Installation

### 1. Generate worker token from control plane

On your Linux control plane node, generate a worker token:

```shell
k0s token create --role=worker > worker-token.txt
```

Transfer this `worker-token.txt` file to your Windows node at `C:\k0s-token.txt`.

### 2. Enable Windows Containers feature

Open PowerShell as Administrator on the Windows node and run:

```powershell
Install-WindowsFeature -Name Containers
```

**Note**: A system reboot is required after enabling this feature.

### 3. Download and install k0s

After rebooting, open PowerShell as Administrator and run:

```powershell
# Set variables
$K0S_VERSION = "v1.34.3+k0s.0"
$TOKEN_FILE = "C:\k0s-token.txt"

# Download k0s
$encodedVersion = $K0S_VERSION.Replace('+', '%2B')
$url = "https://github.com/k0sproject/k0s/releases/download/$encodedVersion/k0s-$K0S_VERSION-amd64.exe"
Invoke-WebRequest -Uri $url -OutFile "$env:LOCALAPPDATA\Microsoft\WindowsApps\k0s.exe"

# Install and start k0s worker
k0s install worker --token-file $TOKEN_FILE
k0s start
```

**Note**: k0s supervises `kubelet.exe` and `kube-proxy.exe`.

## Configuration

### Strict-affinity

You must enable strict affinity to run the Windows node.

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