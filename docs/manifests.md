# Manifest deployer

k0s embeds a manifest deployer on controllers which provides an easy way to deploy manifests automatically.

By default k0s reads all manifests under `${DATADIR}/manifests` (default: `/var/lib/k0s/manifests`) and ensures their state matches the cluster state. When you remove a manifest file, k0s will automatically prune all the resources associated with it.

Each directory that is a **direct descendant** of `${DATADIR}/manifests` is considered
as its own "stack", but nested directories will be excluded from the stack mechanism.

**Note:** k0s uses this mechanism for some of its internal in-cluster components and other resources. Make sure you only touch the manifests not managed by k0s.

## Future

We may in the future support nested directories, but those will not be considered
_stacks_, but rather sub-resources of a parent stacks. Stacks are exclusively top-level.

