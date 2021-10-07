# Dynamic configuration

k0s comes with the option to enable dynamic configuration for cluster level components. This covers all the components other than etcd (or sqlite) and the Kubernetes api-server. This option enables k0s configuration directly via Kubernetes API as opposed to using a configuration file for all cluster configuration.

This feature needs to be separately enabled for all controllers using `k0s controller --enable-dynamic-config`.

## Dynamic vs. static configuration

The existing and enabled-by-default method is what we call static configuration. That's the way where the k0s process reads the config from the given YAML file (or uses the default config if no config is given by user) and configures every component accordingly. This means that for any configuration change the cluster admin needs to restart all controllers on the cluster and have matching configs on each controller node.

In dynamic configuration mode the first controller to boot up when the cluster is created will use the given config YAML as a bootstrap configuration and stores it in the Kubernetes API. All the other controllers will find the config existing on the API and will use it as the source-of-truth for configuring all the components except for etcd and kube-apiserver. After the initial cluster bootstrap the source of truth for all controllers is the configuration object in the Kubernetes API.

## Configuration location

The cluster wide configuration is stored in the Kubernetes API as a custom resource called `clusterconfig`. There's currently only one instance named `k0s`. You can edit the configuration with what ever means possible, for example with:

```shell
kubectl -n kube-system edit clusterconfig k0s
```

This will open the configuration object for editing in our systems default editor.

## Configuration reconciliation

The dynamic configuration uses the typical operator pattern for operation. k0s controller will detect when the object changes and will reconcile the configuration changes to be reflected how different components are configured. So say you want to change the MTU setting for kube-router CNI networking you'd change the config to contain e.g.:

```yaml
    kuberouter:
      mtu: 1350
      autoMTU: false
```

This will change the kube-router related configmap and thus make kube-router to use different MTU settings for new pods.

## Configuration options

The configuration object is a 1-to-1 mapping with the existing [configuration YAML](configuration.md). All the configuration options EXCEPT options under `spec.api` and `spec.storage` are dynamically reconciled.

As with any Kubernetes cluster there are certain things that just cannot be changed on-the-fly such as pod and service CIDRs.

## Configuration status

The dynamic configuration reconciler operator will write status events for all the changes it detects. To see all related events you can query the events where the source object is this k0s config object:

```shell
bash-5.1# k0s kc -n kube-system get event --field-selector involvedObject.name=k0s
LAST SEEN   TYPE      REASON                OBJECT              MESSAGE
64s         Warning   FailedReconciling     clusterconfig/k0s   failed to validate config: [invalid pod CIDR invalid ip address]
59s         Normal    SuccessfulReconcile   clusterconfig/k0s   Succesfully reconciler cluster config
69s         Warning   FailedReconciling     clusterconfig/k0s   cannot change CNI provider from kuberouter to calico
```
