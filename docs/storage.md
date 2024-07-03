# Storage

## CSI

k0s supports a wide range of different storage options by utilizing Container Storage Interface (CSI). All Kubernetes storage solutions are supported and users can easily select the storage that fits best for their needs.

When the storage solution implements CSI, kubernetes can communicate with the storage to create and configure persistent volumes. This makes it easy to dynamically provision the requested volumes. It also expands the supported storage solutions from the previous generation, in-tree volume plugins. More information about the CSI concept is described on the [Kubernetes Blog](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/).

![k0s storage](img/k0s_storage.png)

### Installing 3rd party storage solutions

Follow your storage driver's installation instructions. Note that by default the Kubelet installed by k0s uses a slightly different path for its working directory (`/varlib/k0s/kubelet` instead of `/var/lib/kubelet`). Consult the CSI driver's configuration documentation on how to customize this path. The actual path can differ if you defined the flag `--data-dir`.

## Example storage solutions

Different Kubernetes storage solutions are explained in the [official Kubernetes storage documentation](https://kubernetes.io/docs/concepts/storage/volumes/). All of them can be used with k0s. Here are some popular ones:

- Rook-Ceph (Open Source)
- MinIO (Open Source)
- Gluster (Open Source)
- Longhorn (Open Source)
- Amazon EBS
- Google Persistent Disk
- Azure Disk
- Portworx

If you are looking for a fault-tolerant storage with data replication, you can find a k0s tutorial for configuring Ceph storage with Rook [in here](examples/rook-ceph.md).
