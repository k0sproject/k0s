# Example: Autopilot Workers in a Pod
**Autopilot** has the flexibility to be run in Kubernetes Pods, provided that
a few conditions are met.

An example configuration is listed in this directory, specifically highlighting:
* A `daemonset` that results in an **autopilot** pod created on every `daemonset` node.
* The **autopilot** pod has access to the hosts PID space (`hostPID=true`)
* The **autopilot** container runs in `privileged` mode in order send a `SIGTERM` signal to `k0s` (to restart).
* Required paths from the host are mounted into the **autopilot** Pod:
  * `/usr/local/bin` for downloads + `k0s` renaming
  * `/run/k0s` for reading the `k0s` status socket for the `k0s` PID.


Here is an example `Plan` for `linux-amd64` that will target **all** `linux`
worker nodes.

```
apiVersion: autopilot.k0sproject.io/v1beta2
kind: Plan
metadata:
  name: autopilot

spec:
  id: id1234
  timestamp: now

  commands:
    - k0supdate:
        version: v1.22.5+k0s.1
        platforms:
          linux-amd64:
            url: https://github.com/k0sproject/k0s/releases/download/v1.22.5%2Bk0s.1/k0s-v1.22.5+k0s.1-amd64
            sha256: ceb044963513e780170230b02f6df06e53cca54b3a7ec20c0a41683aabd1ed3a

        targets:
          workers:
            discovery:
              selector:
                labels: kubernetes.io/os=linux
```