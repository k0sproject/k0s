# Windows support
This branch is under active development.
The work for the windows nodes support is in an active phase and this functionality is extremely not ready.

## Build

`TARGET_OS=windows make clean k0s.exe`

This should create k0s.exe with staged kubelet.exe


## Running

It is expected to have docker EE installed on the windows node.

```
C:\>k0s.exe worker --cri-socket=docker:tcp://127.0.0.1:2375 --profile default-windows <token>
```

TODO:
- make node schedulable (so far pods can't start and go to crashloop)
- make calico support
- add selectors to the nodes and system pods (do not run coredns, kube-proxy on windows, maybe some other as well)
- stage containerd and use it instead of cri-runtime

### Hacks

The `kubelet.conf` created by kubelet.conf during the bootstrap has wrong PEM path, should be fixed as for the `default-auth` 

```
users:
- name: default-auth
  user:
    client-certificate: c:\var\lib\kubelet\pki\kubelet-client-current.pem
    client-key: c:\var\lib\kubelet\pki\kubelet-client-current.pem
```

