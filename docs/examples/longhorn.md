<!--
SPDX-FileCopyrightText: 2025 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Longhorn

This tutorial covers installing Longhorn as a Helm extension on k0s. Longhorn provides distributed block storage for Kubernetes with features like snapshots, backup, and live upgrades.

## Prerequisites

- Install `open-iscsi` on every Linux node (required by Longhorn).
- Ensure the `iscsid` service is enabled and running on each node.
- Make sure nodes have a stable hostname and reachable IPs.

Ubuntu/Debian:

```sh
sudo apt-get update
```

```sh
sudo apt-get install -y open-iscsi
```

RHEL/CentOS/Rocky/Alma:

```sh
sudo yum install -y iscsi-initiator-utils
```

SUSE:

```sh
sudo zypper install -y open-iscsi
```

## Install via k0s Helm extension

Add Longhorn as a Helm chart in your k0s ClusterConfig. Apply this configuration through your normal k0s configuration workflow.

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  extensions:
    helm:
      repositories:
        - name: longhorn
          url: https://charts.longhorn.io
      charts:
        - name: longhorn
          chartname: longhorn/longhorn
          namespace: longhorn-system
          version: v1.10.0
```

Wait for components to become ready:

```sh
k0s kubectl -n longhorn-system get pods
```

## Access the Longhorn UI

You can port-forward the frontend service:

```sh
k0s kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80
```

Then open `http://localhost:8080` in your browser.

## Example: PVC and Deployment

Use this example to create a PVC and mount it in a Deployment using Longhorn.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: app-pvc
  namespace: default
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: longhorn
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
            claimName: app-pvc
```

Apply the manifest:

```sh
k0s kubectl apply -f nginx.yaml
```

Check the PVC and pod status:

```sh
k0s kubectl get pvc,pods
```
