# OpenEBS

This tutorial covers the installation of OpenEBS as a Helm extension, both from
scratch and how to migrate it from a storage extension.

## Installing OpenEBS from scratch

**WARNING**: Do not configure OpenEBS as both a storage extension and a Helm
extension. It's considered an invalid configuration and k0s will entirely ignore
the configuration to prevent accidental upgrades or downgrades. The chart
objects defined in the API will still behave normally.

OpenEBS can be installed as a helm chart by adding it as an extension to your configuration:

```yaml
  extensions:
    helm:
      repositories:
      - name: openebs-internal
        url: https://openebs.github.io/charts
      charts:
      - name: openebs
        chartname: openebs-internal/openebs
        version: "3.9.0"
        namespace: openebs
        order: 1
        values: |
          localprovisioner:
            hostpathClass:
              enabled: true
              isDefaultClass: false
```

If you want OpenEBS to be your default storage class, set `isDefaultClass` to `true`.

## Migrating bundled OpenEBS to helm extension

The bundled OpenEBS extension is already a helm extension installed as a
`chart.helm.k0sproject.io`. For this reason, all we have to do is to remove the
manifests and to clean up the object. However, this must be done in a specific order
to prevent data loss.

**WARNING**: Not following the steps in the precise order presented by the
documentation may provoke data loss.

The first step to perform the migration is to disable the `applier-manager`
component on all controllers. For each controller, restart the controller
with the flag `--disable-components=applier-manager`. If you already had this flag,
set it to `--disable-components=<previous value>,applier-manager`.

Once the `applier-manager` is disabled in every running controller, you need to modify
the configuration to use `external_storage` instead of `openebs_local_storage`.

If you are using [dynamic configuration](../dynamic-configuration.md), you can
change it with this command:

```shell
kubectl patch clusterconfig -n kube-system  k0s --patch '{"spec":{"extensions":{"storage":{"type":"external_storage"}}}}' --type=merge
```

If you are using a static configuration file, replace `spec.extensions.storage.type`
from `openebs_local_storage` to `external_storage` in all control plane nodes and
restart all the control plane nodes one by one.

When the configuration is set to `external_storage` and the servers are
restarted, you must manage the it as a chart object in the API:

```shell
kubectl get chart -n kube-system k0s-addon-chart-openebs -o yaml
```

First, remove the labels and annotations related to the stack applier:

```shell
k0s kc annotate -n kube-system chart k0s-addon-chart-openebs k0s.k0sproject.io/stack-checksum-
k0s kc label -n kube-system chart k0s-addon-chart-openebs k0s.k0sproject.io/stack-
```

After the annotations and labels are removed, remove the manifest file **on each
controller**. This file is located in
`<k0s-data-dir>/manifests/helm/<number>_helm_extension_openebs.yaml`, which in
most installations defaults to
`/var/lib/k0s/manifests/helm/0_helm_extension_openebs.yaml`.

**WARNING**: Not removing the old manifest file from all controllers may cause
the manifest to be reapplied, reverting your changes and potentially casuing
data loss.

Finally, we want to re-enable the `applier-manager` and restart all controllers
without the `--disable-components=applier-manager` flag.

Once the migration is coplete, you'll be able to update the OpenEBS chart.
Let's take v3.9.0 as an example:

```shell
kubectl patch chart -n kube-system k0s-addon-chart-openebs --patch '{"spec":{"version":"3.9.0"}}' --type=merge
```

## Usage

Once installed, the cluster will have two storage classes available for you to use:

```shell
k0s kubectl get storageclass
```

```shell
NAME               PROVISIONER        RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
openebs-device     openebs.io/local   Delete          WaitForFirstConsumer   false                  24s
openebs-hostpath   openebs.io/local   Delete          WaitForFirstConsumer   false                  24s
```

The `openebs-hostpath` is the storage class that maps to `/var/openebs/local`.

The `openebs-device` is not configured and could be configured by [manifest deployer](../manifests.md) accordingly to the [OpenEBS documentation](https://docs.openebs.io/)

### Example

Use following manifests as an example of pod with mounted volume:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nginx-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: openebs-hostpath
  resources:
    requests:
      storage: 5Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
  labels:
    app: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  strategy:
    type: Recreate
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx
        name: nginx
        volumeMounts:
        - name: persistent-storage
          mountPath: /var/lib/nginx
      volumes:
      - name: persistent-storage
        persistentVolumeClaim:
          claimName: nginx-pvc
```

```shell
k0s kubectl apply -f nginx.yaml
```

```shell
persistentvolumeclaim/nginx-pvc created
deployment.apps/nginx created
bash-5.1# k0s kc get pods
NAME                    READY   STATUS    RESTARTS   AGE
nginx-d95bcb7db-gzsdt   1/1     Running   0          30s
```

```shell
k0s kubectl get pv
```

```shell
NAME                                       CAPACITY   ACCESS MODES   RECLAIM POLICY   STATUS   CLAIM               STORAGECLASS       REASON   AGE
pvc-9a7fae2d-eb03-42c3-aaa9-1a807d5df12f   5Gi        RWO            Delete           Bound    default/nginx-pvc   openebs-hostpath            30s
```
