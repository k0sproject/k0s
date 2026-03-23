# Version Skew Policy

This document describes the supported version skew between k0s components
and between k0s and the embedded Kubernetes components.

k0s follows the [Kubernetes version skew policy][k8s-version-skew] and adds
a few k0s-specific notes on top.

## Kubernetes Version Skew

k0s embeds specific Kubernetes versions and inherits their skew rules.

### Control Plane Components

The core Kubernetes control plane components (`kube-apiserver`,
`kube-controller-manager`, `kube-scheduler`) must be at the **same minor
version** or at most **one minor version** apart:

| Component | Supported skew relative to kube-apiserver |
|-----------|------------------------------------------|
| kube-controller-manager | ≤ 1 minor version older |
| kube-scheduler | ≤ 1 minor version older |
| kube-proxy | ≤ 1 minor version older |
| kubelet | ≤ 3 minor versions older (never newer) |

> **Important:** `kubelet` can be up to **3 minor versions behind**
> `kube-apiserver` at runtime, but must **never be newer** than `kube-apiserver`.

### Worker Node Kubelet

While the Kubernetes project supports kubelet being up to 3 minor versions
behind the control plane, k0s officially supports **1 minor version** of skew
for all components to keep upgrades predictable.

This means a k0s worker running v1.29.x is supported with a control plane
running v1.30.x, but **not** with v1.31.x or newer.

## k0s Component Version Skew

When running a mixed-version cluster during a rolling upgrade:

- The k0s **controller** version must be **≥** the k0s **worker** version.
- You should upgrade controllers **before** workers.
- Major version upgrades (e.g., `v1.x` → `v2.x`) are not supported. k0s
  currently only has a `v1` API.

## Upgrade Path

k0s supports upgrading **one minor version at a time**:

```
v1.28.x → v1.29.x → v1.30.x → ...
```

Skipping minor versions (e.g., `v1.28.x → v1.30.x` directly) is **not
officially supported** and may cause issues. Always upgrade through each
intermediate minor version.

Use the **latest patch release** of each minor version when upgrading:

```
v1.28.5 → v1.29.8 → v1.30.3 → ...
                ↑
           latest patch of each minor
```

### Autopilot-Managed Upgrades

When using [Autopilot](autopilot.md) for automated upgrades, it enforces the
one-minor-version step rule automatically. Attempting to jump multiple minor
versions in an Autopilot plan will be rejected.

If the kubelet on a worker is no longer reporting after a control plane upgrade,
it is likely running a version that is too far behind the new control plane.
Upgrade the worker to match the control plane's minor version (or within 1
minor version).

## Checking Component Versions

Use these commands to inspect the current versions of your k0s components:

```shell
# Check k0s version on a controller or worker node
k0s version

# Check Kubernetes component versions (on a controller)
k0s kubectl version

# Check all node Kubernetes versions
k0s kubectl get nodes -o wide
```

## Summary Table

| Scenario | Supported |
|----------|-----------|
| Controller n.x, worker n.x | ✅ Yes |
| Controller n+1.x, worker n.x | ✅ Yes (during rolling upgrade) |
| Controller n.x, worker n+1.x | ❌ No (worker must not be newer) |
| Controller n+2.x, worker n.x | ❌ No (too many minor versions) |
| Skip minor version (n.x → n+2.x) | ❌ Not supported |
| Major version upgrade | ❌ Not applicable (only v1 exists) |

## References

- [Kubernetes version skew policy][k8s-version-skew]
- [k0s Autopilot documentation](autopilot.md)
- [k0s upgrade documentation](upgrade.md)

[k8s-version-skew]: https://kubernetes.io/releases/version-skew-policy/
