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

Please refer to the [official Longhorn v1.10.x Support Matrix](https://www.suse.com/suse-longhorn/support-matrix/all-supported-versions/longhorn-v1-10-x/) for detailed system requirements.

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
          version: 1.10.1
```

### Verification Steps

After applying the configuration, verify that Longhorn is properly installed:

**Check pod status**: Wait for all Longhorn components to become ready:

```sh
k0s kubectl -n longhorn-system get pods
```

Expected output should show all pods in `Running` status. This may take 2-3 minutes.

**Verify StorageClass**: Confirm that the `longhorn` StorageClass was created:

```sh
$ k0s kubectl get storageclass
NAME                 PROVISIONER          RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
longhorn (default)   driver.longhorn.io   Delete          Immediate           true                   16m
longhorn-static      driver.longhorn.io   Delete          Immediate           true                   16m
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

## Next Steps

Longhorn supports backing up volumes to external storage systems such as S3, NFS, or CIFS. To enable backups:

1. Create a backup target secret with your storage credentials
2. Configure the backup target URL in Longhorn settings
3. Set up recurring backup jobs for automatic protection

For detailed instructions on configuring backup targets and setting up automated backups, refer to the [Longhorn Backup Documentation](https://longhorn.io/docs/1.10.1/snapshots-and-backups/backup-and-restore/set-backup-target/).

## Additional Resources

- [Longhorn Official Documentation](https://longhorn.io/docs/)
- [Longhorn Architecture Guide](https://longhorn.io/docs/1.10.1/concepts/)
- [Best Practices for Production Deployments](https://longhorn.io/docs/1.10.1/best-practices/)
- [Troubleshooting Guide](https://longhorn.io/docs/1.10.1/troubleshoot/troubleshooting/)
