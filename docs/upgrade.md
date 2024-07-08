# Upgrade

The k0s upgrade is a simple process due to its single binary distribution. The k0s single binary file includes all the necessary parts for the upgrade and essentially the upgrade process is to replace that file and restart the service.

This tutorial explains two different approaches for k0s upgrade:

- [Upgrade a k0s node locally](#upgrade-a-k0s-node-locally)
- [Upgrade a k0s cluster using k0sctl](#upgrade-a-k0s-cluster-using-k0sctl)

## Upgrade a k0s node locally

If your k0s cluster has been deployed with k0sctl, then k0sctl provides the easiest upgrade method. In that case jump to the next chapter. However, if you have deployed k0s without k0sctl, then follow the upgrade method explained in this chapter.

Before starting the upgrade, consider moving your applications to another node if you want to avoid downtime. This can be done by [draining a worker node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/). Remember to uncordon the worker node afterwards to tell Kubernetes that it can resume scheduling new pods onto the node.

The upgrade process is started by stopping the currently running k0s service.

```shell
sudo k0s stop
```

Now you can replace the old k0s binary file. The easiest way is to use the download script. It will download the latest k0s binary and replace the old binary with it. You can also do this manually without the download script.

```shell
curl --proto '=https' --tlsv1.2 -sSf https://get.k0s.sh | sudo sh
```

Then you can start the service (with the upgraded k0s) and your upgrade is done.

```shell
sudo k0s start
```

## Upgrade a k0s cluster using k0sctl

The upgrading of k0s clusters using k0sctl occurs not through a particular command (there is no `upgrade` sub-command in k0sctl) but by way of the configuration file. The configuration file describes the desired state of the cluster, and when you pass the description to the `k0sctl apply` command a discovery of the current state is performed and the system does whatever is necessary to bring the cluster to the desired state (for example, perform an upgrade).

### k0sctl cluster upgrade process

The following operations occur during a k0sctl upgrade:

1. Upgrade of each controller, one at a time. There is no downtime if multiple controllers are configured.

2. Upgrade of workers, in batches of 10%.

3. Draining of workers, which allows the workload to move to other nodes prior to the actual upgrade of the worker node components. (To skip the drain process, use the ``--no-drain`` option.)

4. The upgrade process continues once the upgraded nodes return to **Ready** state.

You can configure the desired cluster version in the k0sctl configuration by setting the value of `spec.k0s.version`:

```yaml
spec:
  k0s:
    version: {{{ extra.k8s_version }}}+k0s.0
```

If you do not specify a version, k0sctl checks online for the latest version and defaults to it.

```shell
k0sctl apply
```

```shell
...
...
INFO[0001] ==> Running phase: Upgrade controllers
INFO[0001] [ssh] 10.0.0.23:22: starting upgrade
INFO[0001] [ssh] 10.0.0.23:22: Running with legacy service name, migrating...
INFO[0011] [ssh] 10.0.0.23:22: waiting for the k0s service to start
INFO[0016] ==> Running phase: Upgrade workers
INFO[0016] Upgrading 1 workers in parallel
INFO[0016] [ssh] 10.0.0.17:22: upgrade starting
INFO[0027] [ssh] 10.0.0.17:22: waiting for node to become ready again
INFO[0027] [ssh] 10.0.0.17:22: upgrade successful
INFO[0027] ==> Running phase: Disconnect from hosts
INFO[0027] ==> Finished in 27s
INFO[0027] k0s cluster version {{{ extra.k8s_version }}}+k0s.0 is now installed
INFO[0027] Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO[0027]      k0sctl kubeconfig
```
