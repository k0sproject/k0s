# Windows support

## Experimental status

Windows support feature is under active development and MUST BE considered as highly experemential. 


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
C:\>k0s.exe worker --cri-socket=docker:tcp://127.0.0.1:2375 --api-server=<k0s api> <token>
```

