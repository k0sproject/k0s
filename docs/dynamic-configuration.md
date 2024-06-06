# Dynamic configuration

k0s comes with the option to enable dynamic configuration for cluster level components. This covers all the components other than etcd (or sqlite) and the Kubernetes api-server. This option enables k0s configuration directly via Kubernetes API as opposed to using a configuration file for all cluster configuration.

This feature has to be enabled for every controller in the cluster using the `--enable-dynamic-config` flag in `k0s controller` or `k0s install controller` commands. Having both types of controllers in the same cluster will cause a conflict.

## Dynamic vs. static configuration

The existing and enabled-by-default method is what we call static configuration. That's the way where the k0s process reads the config from the given YAML file (or uses the default config if no config is given by user) and configures every component accordingly. This means that for any configuration change the cluster admin has to restart all controllers on the cluster and have matching configs on each controller node.

In dynamic configuration mode the first controller to boot up when the cluster is created will use the given config YAML as a bootstrap configuration and stores it in the Kubernetes API. All the other controllers will find the config existing on the API and will use it as the source-of-truth for configuring all the components except for etcd and kube-apiserver. After the initial cluster bootstrap the source of truth for all controllers is the configuration object in the Kubernetes API.

## Cluster configuration vs. controller node configuration

In the [k0s configuration options](configuration.md) there are some options that are cluster-wide and some that are specific to each controller node in the cluster. The following list outlines which options are controller node specific and have to be configured only via the local file:

- `spec.api` - these options configure how the local Kubernetes API server is setup
- `spec.storage` - these options configure how the local storage (etcd or sqlite) is setup
- `spec.network.controlPlaneLoadBalancing` - these options configure how [Control Plane Load Balancing](cplb.md) is setup.

In case of HA control plane, all the controllers will need this part of the configuration as otherwise they will not be able to get the storage and Kubernetes API server running.

## Configuration location

The cluster wide configuration is stored in the Kubernetes API as a custom resource called `clusterconfig`. There's currently only one instance named `k0s`. You can edit the configuration with what ever means possible, for example with:

```shell
k0s config edit
```

This will open the configuration object for editing in your system's default editor.

## Configuration reconciliation

The dynamic configuration uses the typical operator pattern for operation. k0s controller will detect when the object changes and will reconcile the configuration changes to be reflected to how different components are configured. So say you want to change the MTU setting for kube-router CNI networking you'd change the config to contain e.g.:

```yaml
    kuberouter:
      mtu: 1350
      autoMTU: false
```

This will change the kube-router related configmap and thus make kube-router to use different MTU settings for new pods.

## Configuration options

The configuration object is a 1-to-1 mapping with the existing [configuration YAML](configuration.md). All the configuration options EXCEPT options under `spec.api` and `spec.storage` are dynamically reconciled.

As with any Kubernetes cluster there are certain things that just cannot be changed on-the-fly, this is the list of non-changeable options:

- `network.podCIDR`
- `network.serviceCIDR`
- `network.provider`
- `network.controlPlaneLoadBalancing`

During the manual installation of control plane nodes with `k0s install`, all these
non-changeable options must be defined in the configuration file. This is necessary
because these fields can be used before the dynamic configuration reconciler is
initialized. Both k0sctl and k0smotron handle this without user intervention.

## Configuration status

The dynamic configuration reconciler operator will write status events for all the changes it detects. To see all dynamic config related events, use:

```shell
k0s config status
```

```shell
LAST SEEN   TYPE      REASON                OBJECT              MESSAGE
64s         Warning   FailedReconciling     clusterconfig/k0s   failed to validate config: [invalid pod CIDR invalid ip address]
59s         Normal    SuccessfulReconcile   clusterconfig/k0s   Succesfully reconciler cluster config
69s         Warning   FailedReconciling     clusterconfig/k0s   cannot change CNI provider from kuberouter to calico
```
