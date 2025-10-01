<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Releases

This page describes how we release and support the k0s project. [Mirantis Inc.](https://mirantis.com) can also provide [commercial support](commercial-support.md) for k0s.

## Upstream Kubernetes release & support cycle

{% set version_parts = k0s_version.split('.', maxsplit=2) -%}
{% set k0s_minor = version_parts[0] + '.' + version_parts[1] -%}
{% set k0s_prev_minor = version_parts[0] + '.' ~ ((version_parts[1] | int) - 1) -%}

This [release and support cycle](https://kubernetes.io/releases/) is followed
for ALL new minor releases. A minor release can be e.g. {{{ k0s_prev_minor }}},
{{{ k0s_minor }}} and so on. What this means in practice is that every 4 months
there is a new minor release published.

After a minor release is published, the upstream community is maintaining it for 14 months. Maintenance in this case means that upstream Kubernetes provides bug fixes, CVE mitigations and such for 14 months per minor release.

![Kubernetes release and support cycle](img/k8s_release_cycle.png)

## k0s release and support model

Starting from the k0s 1.21, k0s started following the Kubernetes project's [release and support model](https://kubernetes.io/releases/).

k0s project follows closely the upstream Kubernetes release cycle. The only difference to upstream Kubernetes release / maintenance schedule is that our initial release date is always a few weeks behind the upstream Kubernetes version release date as we are building our version of k0s from the officially released version of Kubernetes and need time for testing the final version before shipping.

![k0s release model](img/k0s_releases.png)

Given the fact that upstream Kubernetes provides support and patch releases for a minor version for roughly 14 months, it means that k0s will follow this same model. Each minor release is maintained for roughly 14 months since its initial release.

k0s project will typically include patches and fixes included in a Kubernetes
upstream patch release for the fixes needed in k0s own codebase. For example, if
a bug is identified in {{{ k0s_minor }}} series k0s project will create and ship
a fix for it with the next upstream Kubernetes {{{ k0s_minor }}}.x release. In
rare cases where a critical bug is identified we may also ship "out of band"
patches. Such out-of-band release would be identified in the version string
suffix. For example a normal release following Kubernetes upstream would be {{{
k0s_minor }}}.0+k0s.0 whereas a critical out-of-band patch would be identified as
{{{ k0s_minor }}}.0+k0s.1.

## New features and enhancements

The biggest new k0s features will typically only be delivered on top of the latest Kubernetes version, but smaller enhancements can be included in older release tracks as well.

## Version string

{% set version_parts = k0s_version.split('+', maxsplit=1) -%}

The k0s version string consists of the Kubernetes version and the k0s version. For example:

```text
{{{ version_parts | join('+') }}}
```

The Kubernetes version (`{{{ version_parts[0] }}}`) is the first part, and the
last part (`{{{ version_parts[1] }}}`) reflects the k0s version, which is built
on top of the certain Kubernetes version.
