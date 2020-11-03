# Manifest deployer

k0s embeds a manifest deployer on controllers which allows easy way to deploy manifests automatically. By default k0s reads all manifests in `/var/lib/k0s/manifests` and ensures their state matches on the cluster. When you remove a manifest file k0s will automatically prune all the resources associated with it.

Each directory that is a **direct descendant** of `/var/lib/k0s/manifests` is considered
to be its own stack, but nested directories are not considered new stacks.

**Note:** k0s uses this mechanism for some of it's internal in-cluster components and other resources. Make sure you only touch the manifests not managed by k0s.

## Future

We may in the future support nested directories, but those will not be considered
_stacks_, but rather subresources of a parent stack. Stacks are exclusively top-level.

