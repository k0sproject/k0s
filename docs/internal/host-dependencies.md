# Host Dependencies

The goal of MKE is to only depend on kernel functionality. However, the kubelet
currently has several host dependencies. Some of these may need to be bundled,
but we should prefer upstream contributions to remove these dependencies.

## List of hard dependencies

- `find`
-- PR by @ncopa to resolve this: https://github.com/kubernetes/kubernetes/pull/95189
- `du`
-- PR by @ncopa to resolve this: https://github.com/kubernetes/kubernetes/pull/95178
-- note that `du` dependency remains, but using POSIX-compliant argument 
- `nice`
- `iptables`
-- as documented in https://github.com/Mirantis/mke/issues/176 it is unclear whether `iptables` is needed. It appears to come from the `portmap` plugin, but the most robust solution may be to simply bundle `iptables` with MKE.
