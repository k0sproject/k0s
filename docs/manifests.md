# Manifest Deployer

To run k0s with your preferred extensions you have two options.

1. Define your extensions as Helm charts as described [in here](helm-charts.md).

2. Use Manifest Deployer, which is included into k0s. This option is described on below.

### Overview

k0s embeds Manifest Deployer, which runs on the controller nodes and provides an easy way to deploy manifests automatically at runtime. 

By default, k0s reads all manifests under `/var/lib/k0s/manifests` and ensures that their state matches the cluster state. Moreover, when a manifest file is removed, k0s will automatically prune all the resources associated with it. 

For the most part, Manifest Deployer can be used like the `kubectl apply` command. The main difference is that Manifest Deployer constantly monitors the directory for changes. Thus, users don't need to manually apply the changes that are made to the manifest files.

##### Note

- Each directory that is a **direct descendant** of `/var/lib/k0s/manifests` is considered as its own "stack", but nested directories (further subfolders) will be excluded from the stack mechanism and thus, they are not automatically deployed by the Manifest Deployer.
- k0s uses this mechanism for some of its internal in-cluster components and other resources. Make sure you only touch the manifests not managed by k0s.
- Namespace must be explicitly defined in the manifests. There's no default namespace used by Manifest Deployer.

### Example

You can try Manifest Deployer by creating a new folder under `/var/lib/k0s/manifests` and then create a manifest file like `nginx.yaml` with the following content:

```sh
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

You should soon see the new pods appearing:
```sh
$ sudo k0s kubectl get pods --namespace nginx
NAME                                READY   STATUS    RESTARTS   AGE
nginx-deployment-66b6c48dd5-8zq7d   1/1     Running   0          10m
nginx-deployment-66b6c48dd5-br4jv   1/1     Running   0          10m
nginx-deployment-66b6c48dd5-sqvhb   1/1     Running   0          10m
```
