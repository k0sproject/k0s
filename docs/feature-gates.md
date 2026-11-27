<!--
SPDX-FileCopyrightText: 2025 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Feature Gates

Feature gates are a mechanism to enable or disable experimental features in k0s.
They allow you to control which features are active in your cluster, providing
a way to safely test new functionality before it becomes generally available.

## Overview

Feature gates in k0s follow a lifecycle model similar to Kubernetes:

- **Alpha**: Experimental features that are disabled by default and may be unstable or removed
- **Beta**: Features that are more stable but still being tested, enabled by default
- **GA (Generally Available)**: Stable features that cannot be disabled. If explicitly disabled
k0s will fail to start

Once a feature reaches GA status, the feature gate cannot be disabled anymore and it will be
eventually removed.

## Using Feature Gates

Feature gates can be configured using the `--feature-gates` flag when starting k0s controllers
and workers:

```shell
k0s controller --feature-gates="ExampleFeature=true,AnotherFeature=false"
k0s worker --feature-gates="ExampleFeature=true"
```

The feature gates configuration must be consistent across all controllers in the cluster. Gates
that affect workers have to be set on each worker that should use them. Controllers running a
worker (`--enable-worker` or `--single`) apply their feature gates to that worker as well.

**Important notes:**

- Setting a GA feature gate to false will cause k0s startup to fail
- Unknown feature gates will cause k0s start up to fail
- Feature gates use case-sensitive CamelCase names

## Available Features

The following table is a summary of the feature gates that you can set. Each feature is designed for a specific stage of stability:

| Feature           | Default | Stage | Since | Until |
|-------------------|---------|-------|-------|-------|
| `IPv6SingleStack` | `false` | Alpha | 1.34  |       |

### Feature Details

- `IPv6SingleStack`: Enables single-stack IPv6 support in k0s clusters.
