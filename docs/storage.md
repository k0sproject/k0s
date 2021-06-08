# Storage (CSI)

k0s supports a wide range of different storage options. There are no "selected" storage in k0s. Instead, all Kubernetes storage solutions are supported and users can easily select the storage that fits best for their needs.

When the storage solution implements Container Storage Interface (CSI), containers can communicate with the storage for creation and configuration of persistent volumes. This makes it easy to dynamically provision the requested volumes. It also expands the supported storage solutions from the previous generation, in-tree volume plugins. More information about the CSI concept is described on the [Kubernetes Blog](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/).

![k0s storage](img/k0s_storage.png)

## Example storage solutions

Different Kubernetes storage solutions are explained in the [official Kubernetes storage documentation](https://kubernetes.io/docs/concepts/storage/volumes/). All of them can be used with k0s. Here are some popular ones:

- Rook-Ceph (Open Source)
- OpenEBS (Open Source)
- MinIO (Open Source)
- Gluster (Open Source)
- Longhorn (Open Source)
- Amazon EBS
- Google Persistent Disk
- Azure Disk
- Portworx

If you are looking for a fault-tolerant storage with data replication, you can find a k0s tutorial for configuring Ceph storage with Rook [in here](examples/rook-ceph.md).

If you are looking for a bit more simple solution and use a folder from the node local disk, you can take a look at [OpenEBS](https://docs.openebs.io/docs/next/uglocalpv-hostpath.html). With OpenEBS, you can either create a simple local storage or a highly available distributed storage.
