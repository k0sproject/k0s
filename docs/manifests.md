# Manifest deployer

MKE embeds a manifest deployer on controllers which allows easy way to deploy manifests automatically. By default MKE reads all manifests in `/var/lib/mke/manifests` and ensures their state matches on the cluster. When you remove a manifest file MKE will automatically prune all the resources associated with it.

**Note:** MKE uses this mechanism for some of it's internal in-cluster components and other resources. Make sure you only touch the manifests not managed by MKE.

## Future

We intend to re-write the manifest handling partially. Currently all manifests are managed as a single "stack", meaning they will automatically get some of the same labels etc to allow automated pruning. We're planning to make this work in a stack per directory basis.

