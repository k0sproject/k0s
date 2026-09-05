<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Version skew policy

k0s ships a curated set of Kubernetes components (kube-apiserver,
kube-controller-manager, kube-scheduler, kubelet, kube-proxy, …) for each
release, so the version skew between those components inside a single k0s
release is always supported by upstream Kubernetes by construction.

What you still control as an operator is the skew **between releases of
k0s** that you mix in one cluster — for example, controllers running k0s
`v1.31.x+k0s.0` while workers still run `v1.30.x+k0s.0` during a rolling
upgrade.

## Supported skew

k0s is designed and tested for clusters in which all nodes run either the same
minor version or two adjacent ones, as happens during a rolling upgrade. In
practice, that means:

- **Controllers** must all be within **one minor** of each other.
- **Workers** may be **one minor older** than the controllers, but **must
  never be newer**.

Patch-version differences inside the same minor are always supported and are the
expected state during a phased upgrade.

Note that the upstream Kubernetes [version skew policy] permits wider skews,
such as kubelets up to three minors older than the API server. k0s doesn't
support those. Each k0s version is only expected to interoperate with the
previous one, and upgrades proceed one minor at a time.

[version skew policy]: https://kubernetes.io/releases/version-skew-policy/

## Upgrades

When upgrading k0s in place:

1. Upgrade controllers first, **one minor at a time** (e.g. v1.30 → v1.31,
   not v1.30 → v1.32).
2. Upgrade workers after the control plane is fully on the new minor.
3. Consider moving to the latest patch release of the old minor first. This
   isn't required, but it takes the old minor's latest fixes along into the
   upgrade.

[Autopilot], [k0sctl], and [k0smotron] all upgrade controllers before workers.
When upgrading manually, keep to the same order.

[Autopilot]: autopilot.md
[k0sctl]: k0sctl-install.md
[k0smotron]: https://github.com/k0sproject/k0smotron

## Downgrades

Downgrades are **not** supported by upstream Kubernetes and are not tested
by k0s. If a release needs to be rolled back, restore from a
[backup](backup.md) taken before the upgrade rather than running an older
k0s binary against a newer cluster state.

## Major versions

There is currently no Kubernetes v2 and no k0s v2, so major-version skew is
not a configuration that exists in practice.
