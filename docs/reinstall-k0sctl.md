# Reinstall a node

`k0sctl` currently does not support changing all the configuration of containerd (`state`, `root`) on the fly.

For example, in order to move containerd's `root` directory to a new partition/drive, you have to provide `--data-dir /new/drive` in your k0sctl `installFlags` for each (worker) node. `--data-dir` is an option of `k0s` and then added to the service unit.

The following is an example of that:

```yaml
# spec.hosts[*].installFlags
  - role: worker
    installFlags:
      - --profile flatcar
      - --enable-cloud-provider
      - --data-dir /new/drive
      - --kubelet-extra-args="--cloud-provider=external"
```

However, the `installFlags` are only used when the node is installed.

## Steps

Drain the node:

```shell
kubectl drain node.hostname
```

Access your node (e.g. via ssh) to stop and reset k0s:

```shell
sudo k0s stop
sudo k0s reset
```

Reboot the node (for good measure):

```shell
sudo systemctl reboot
```

Once the node is available again, run `k0sctl apply` to integrate it into your cluster and uncordon the node to allow pods to be scheduled:

```shell
k0sctl apply -c config.yaml
kubectl uncordon node.hostname
```
