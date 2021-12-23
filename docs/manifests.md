# Manifest Deployer

Included with k0s, Manifest Deployer is one of two methods you can use to run k0s with your preferred extensions (the other being by defining your extensions as [Helm charts](helm-charts.md)).

## Overview

Manifest Deployer runs on the controller nodes and provides an easy way to automatically deploy manifests at runtime.

By default, k0s reads all manifests under `/var/lib/k0s/manifests` and ensures that their state matches the cluster state. Moreover, on removal of a manifest file, k0s will automatically prune all of it associated resources.

The use of Manifest Deployer is quite similar to the use the `kubectl apply` command. The main difference between the two is that Manifest Deployer constantly monitors the directory for changes, and thus you do not need to manually apply changes that are made to the manifest files.

### Note

- Each directory that is a direct descendant of `/var/lib/k0s/manifests` is considered to be its own "stack". Nested directories (further subfolders), however, are excluded from the stack mechanism and thus are not automatically deployed by the Manifest Deployer.

- k0s uses the indepenent stack mechanism for some of its internal in-cluster components, as well as for other resources. Be sure to only touch the manifests that are not managed by k0s.

- Explicitly define the namespace in the manifests (Manifest Deployer does not have a default namespace).

## Example

To try Manifest Deployer, create a new folder under `/var/lib/k0s/manifests` and then create a manifest file (such as `nginx.yaml`) with the following content:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: nginx
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
  namespace: nginx
spec:
  selector:
    matchLabels:
      app: nginx
  replicas: 3
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:latest
        ports:
        - containerPort: 80
```

New pods will appear soon thereafter.

```shell
sudo k0s kubectl get pods --namespace nginx
```

```shell
NAME                                READY   STATUS    RESTARTS   AGE
nginx-deployment-66b6c48dd5-8zq7d   1/1     Running   0          10m
nginx-deployment-66b6c48dd5-br4jv   1/1     Running   0          10m
nginx-deployment-66b6c48dd5-sqvhb   1/1     Running   0          10m
```
