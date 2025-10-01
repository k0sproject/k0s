<!--
SPDX-FileCopyrightText: 2024 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Logs

k0s runs most of the Kubernetes and other system components as plain Linux child processes. As it acts as a watchdog for the child processes it also combines the logs for all such processes into its own log stream.

k0s adds a "selector" to each components log stream so it is easier to distinguish logs from various components. For example the log stream from kubelet is enhanced with a selector `component=kubelet`:

```text
Jul 08 08:46:25 worker-kppzr-lls2q k0s[1766]: time="2024-07-08 08:46:25" level=info msg="I0708 08:46:25.876821    1814 operation_generator.go:721] \"MountVolume.SetUp succeeded for volume \\\"kube-api-access-7tfxw\\\" (UniqueName: \\\"kubernetes.io/projected/ca514728-a1de-4408-9be5-8b36ee896752-kube-api-access-7tfxw\\\") pod \\\"node-shell-a16894ee-eb67-4865-8964-44ca5c87e18d\\\" (UID: \\\"ca514728-a1de-4408-9be5-8b36ee896752\\\") \" pod=\"kube-system/node-shell-a16894ee-eb67-4865-8964-44ca5c87e18d\"" component=kubelet stream=stderr
Jul 08 08:46:26 worker-kppzr-lls2q k0s[1766]: time="2024-07-08 08:46:26" level=info msg="I0708 08:46:26.112550    1814 kuberuntime_container_linux.go:167] \"No swap cgroup controller present\" swapBehavior=\"\" pod=\"kube-system/node-shell-a16894ee-eb67-4865-8964-44ca5c87e18d\" containerName=\"shell\"" component=kubelet stream=stderr
```

## Where are the logs?

### systemd based setups

systemd stores logs in its journal. Access k0s and all component logs using
`journalctl`. For example, to view kubelet logs, run `journalctl -u k0sworker |
grep component=kubelet`.

### OpenRC based setups

openRC stores service logs in `/var/log/k0sworker.log`. To view a specific component log, run `grep component=kubelet /var/log/k0s.log`.
