# Windows support

The work for the windows nodes support is in active phase and this functionality is extremely not done.

## Build

`TARGET_OS=windows make clean k0s.exe`

This should create k0s.exe with staged kubelet.exe


## Running

It is expected to have docker EE installed on the windows node.

```
C:\>k0s.exe worker --cri-socket=docker:tcp://127.0.0.1:2375 --profile default-windows <token>
```