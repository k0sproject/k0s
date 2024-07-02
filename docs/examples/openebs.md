# OpenEBS

This tutorial covers the installation of OpenEBS as a Helm extension. OpenEBS
can be installed as a helm chart by adding it as an extension to your
configuration:

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
