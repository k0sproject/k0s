<!--
SPDX-FileCopyrightText: 2025 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Longhorn

This tutorial covers installing Longhorn as a Helm extension on k0s. Longhorn is a distributed block storage system for Kubernetes that provides enterprise-grade features including:

- **Distributed storage**: Provides persistent storage across your cluster nodes
- **Snapshots and backups**: Built-in snapshot and backup capabilities for data protection
- **Live upgrades**: Zero-downtime upgrades of storage volumes
- **High availability**: Automatic failover and replication for data durability
- **Cross-cluster disaster recovery**: Backup to external storage systems

Longhorn is particularly well-suited for stateful applications that require persistent storage, such as databases, message queues, and content management systems.

For more information on Longhorn, refer to the [official documentation](https://longhorn.io/docs/).

## Prerequisites

Before installing Longhorn, ensure your k0s cluster meets these requirements:

### System Requirements

- **Operating System**: Ubuntu 24.04+, SLES 15 SP7+, SLE Micro 6.1+, RHEL 10.0+, Oracle Linux 9.6+, Rocky Linux 10.0+, Talos 1.10.6+, Container-Optimized OS 121+
- **Kernel**: Linux kernel 3.10 or later
- **Storage**: At least 10GB of free disk space per node for Longhorn data
- **Memory**: Minimum 2GB RAM per node (4GB+ recommended for production)
- **Network**: Stable network connectivity between all cluster nodes

For more detailed system requirements, refer to the [official Longhorn v1.10.x Support Matrix](https://www.suse.com/suse-longhorn/support-matrix/all-supported-versions/longhorn-v1-10-x/).

### Required Software

Longhorn requires the iSCSI initiator utilities to be installed on every node in your cluster:

**Why iSCSI is required**: Longhorn uses iSCSI protocol to provide block storage to pods. The iSCSI initiator allows nodes to connect to Longhorn volumes as if they were local block devices.

Ubuntu/Debian:

```sh
sudo apt-get install -y open-iscsi
```

RHEL/CentOS/Rocky:

```sh
sudo yum install -y iscsi-initiator-utils
```

SUSE:

```sh
sudo zypper install -y open-iscsi
```

Ensure the iSCSI daemon is enabled and running:

```sh
sudo systemctl enable --now iscsid
```

## Install via k0s Helm extension

Add Longhorn as a Helm chart in your k0s ClusterConfig. Apply this configuration through your normal k0s configuration workflow.

### Basic Installation

The simplest way to install Longhorn is to add it to your k0s ClusterConfig:

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
          version: "latest"
```

### Verification Steps

After applying the configuration, verify that Longhorn is properly installed:

1. **Check pod status**: Wait for all Longhorn components to become ready:

```sh
k0s kubectl -n longhorn-system get pods
```

Expected output should show all pods in `Running` status. This may take 2-3 minutes.

2. **Verify StorageClass**: Confirm that the `longhorn` StorageClass was created:

```sh
k0s kubectl get storageclass
```

You should see `longhorn` listed as a storage class.

### Single-Node Cluster Configuration

Single-node clusters require setting the default replica count to 1 to prevent scheduling failures, as Longhorn's default of 3 replicas cannot be satisfied on a single node. This setting can be adjusted in the values or post-installation via the UI (Settings > General > Default Replica Count).

```yaml
spec:
  extensions:
    helm:
      charts:
        - name: longhorn
          chartname: longhorn/longhorn
          namespace: longhorn-system
          version: "latest"
          values: |
            defaultSettings:
              defaultReplicaCount: 1
```

## Access the Longhorn UI

You can port-forward the frontend service:

```sh
k0s kubectl -n longhorn-system port-forward svc/longhorn-frontend 8080:80
```

Then open <http://localhost:8080> in your browser.

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

## Advanced Configuration

### Custom Storage Network

Configure Longhorn to use a dedicated storage network:

```yaml
spec:
  extensions:
    helm:
      charts:
        - name: longhorn
          chartname: longhorn/longhorn
          namespace: longhorn-system
          version: "latest"
          values: |
            defaultSettings:
              storageNetwork: "10.0.0.0/24"
```

### Backup Configuration

Configure Longhorn backup to external storage:

```yaml
            defaultSettings:
              backupTarget: "s3://your-bucket@your-region/"
              backupTargetCredentialSecret: "backup-secret"
```

Create the backup secret:
```sh
k0s kubectl create secret generic backup-secret \
  --from-literal=AWS_ACCESS_KEY_ID=your-access-key \
  --from-literal=AWS_SECRET_ACCESS_KEY=your-secret-key \
  -n longhorn-system
```

## Additional Resources

- [Longhorn Official Documentation](https://longhorn.io/docs/)
- [Longhorn Architecture Guide](https://longhorn.io/docs/1.10.1/concepts/)
- [Best Practices for Production Deployments](https://longhorn.io/docs/1.10.1/best-practices/)
- [Troubleshooting Guide](https://longhorn.io/docs/1.10.1/troubleshoot/troubleshooting/)
