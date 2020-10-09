# Manifest deployer

MKE embeds a manifest deployer on controllers which allows easy way to deploy manifests automatically. By default MKE reads all manifests in `/var/lib/mke/manifests` and ensures their state matches on the cluster. When you remove a manifest file MKE will automatically prune all the resources associated with it.

Each directory that is a **direct descendant** of `/var/lib/mke/manifests` is considered
to be its own stack, but nested directories are not considered new stacks.

**Note:** MKE uses this mechanism for some of it's internal in-cluster components and other resources. Make sure you only touch the manifests not managed by MKE.

## Future

We may in the future support nested directories, but those will not be considered
_stacks_, but rather subresources of a parent stack. Stacks are exclusively top-level.

