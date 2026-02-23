<!--
SPDX-FileCopyrightText: 2023 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# OpenEBS

This tutorial covers the installation of OpenEBS as a Helm extension. OpenEBS
can be installed as a helm chart by adding it as an extension to your
configuration.

## Configuration

Most options can be directly set via the `values` section of the chart config.

[Chart in Artifacthub](https://artifacthub.io/packages/helm/openebs/openebs)

### Kubelet path

Be sure to set the correct kubelet path as the default one from the chart is `/var/lib/kubelet`, but k0s uses `/var/lib/k0s/kubelet`.

You can skip this if you set a [custom directory](https://docs.k0sproject.io/stable/configuration/?h=data+dir#kubelet-root-directory) via `--kubelet-root-dir`.

More infos in the [OpenEBS Quickstart Guide](https://openebs.io/docs/quickstart-guide/installation).

```yaml
lvm-localpv:
  lvmNode:
    kubeletDir: /var/lib/k0s/kubelet
zfs-localpv:
  zfsNode:
    kubeletDir: /var/lib/k0s/kubelet
mayastor:
  csi:
    node:
      kubeletDir: /var/lib/k0s/kubelet
```

### Storage engines

Each storage engine can be individually enabled or disabled.

The default of the chart is to enable all engines except "rawfile".
If you want to disable replicated storage via mayastor, check the example below.

You can always check the default values here: https://github.com/openebs/openebs/blob/helm-testing/release/4.4/charts/values.yaml

```yaml
engines:
  local:
    lvm:
      enabled: true
    rawfile:
      enabled: true
    zfs:
      enabled: true
    rawfile:
        enabled: false
  replicated:
    mayastor:
      enabled: true
```

### Monitoring

Per default the chart also installs Loki, Minio and Alloy for logging purposes.
If you do not need these you can disable them.

```yaml
alloy:
  enabled: false
loki:
  enabled: false
  minio:
    enabled: false
```

### Local storage only variant

```yaml
extensions:
  helm:
    repositories:
      - name: openebs-internal
        url: https://openebs.github.io/openebs
    charts:
      - name: openebs
        chartname: openebs-internal/openebs
        version: "4.4.0"
        namespace: openebs
        order: 1
        values: |
          engines:
            replicated:
              mayastor:
                enabled: false
          lvm-localpv:
            lvmNode:
              kubeletDir: /var/lib/k0s/kubelet
          zfs-localpv:
            zfsNode:
              kubeletDir: /var/lib/k0s/kubelet
          localpv-provisioner:
            hostpathClass:
              isDefaultClass: false
```

### Local + replicated storage variant

Be sure to read the [prerequisites](https://openebs.io/docs/quickstart-guide/prerequisites) of OpenEBS for distributed storage, as some host dependencies are needed.

```yaml
extensions:
  helm:
    repositories:
      - name: openebs-internal
        url: https://openebs.github.io/openebs
    charts:
      - name: openebs
        chartname: openebs-internal/openebs
        version: "4.4.0"
        namespace: openebs
        order: 1
        values: |
          lvm-localpv:
            lvmNode:
              kubeletDir: /var/lib/k0s/kubelet
          zfs-localpv:
            zfsNode:
              kubeletDir: /var/lib/k0s/kubelet
          mayastor:
            csi:
              node:
                kubeletDir: /var/lib/k0s/kubelet
          # uncomment this to only use 1 cpu per io-engine (for smaller workloads)
          # io_engine:
          #   cpuCount: "1"  
          localpv-provisioner:
            hostpathClass:
              isDefaultClass: false
```

### Set default storage class

If you want OpenEBS "hostpath" to be your default storage class, set `isDefaultClass` to `true`.

## Usage

Once installed, the cluster will have multiple storage classes available for you to use (depending on the enabled engines).

```shell
k0s kubectl get storageclass
```

```shell
NAME                         PROVISIONER                    RECLAIMPOLICY   VOLUMEBINDINGMODE      ALLOWVOLUMEEXPANSION   AGE
mayastor-etcd-localpv        openebs.io/local                    Delete          WaitForFirstConsumer   false                  76m
openebs-hostpath (default)   openebs.io/local                    Delete          WaitForFirstConsumer   false                  76m

# and depending on the enabled storage providers e.g.:
openebs-single-replica       io.openebs.csi-mayastor             Delete          Immediate              true                   76m
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
