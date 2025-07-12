# Feature Gates

Feature gates are a mechanism to enable or disable experimental features in k0s.
They allow you to control which features are active in your cluster, providing
a way to safely test new functionality before it becomes generally available.

## Overview

Feature gates in k0s follow a lifecycle model similar to Kubernetes:

- **Alpha**: Experimental features that are disabled by default and may be unstable
- **Beta**: Features that are more stable but still being tested, disabled by default
- **GA (Generally Available)**: Stable features that are enabled by default

## Using Feature Gates

Feature gates can be configured using the `--feature-gates` flag when starting the k0s controller:

```shell
k0s controller --feature-gates="FeatureName=true,AnotherFeature=false"
```

Feature gates are case sensitive.

### Global Feature Gates

k0s supports global toggles for feature maturity levels:

- `AllAlpha=true`: Enables all alpha features
- `AllBeta=true`: Enables all beta features

Individual feature settings override global settings:

```bash
# Enable all alpha features except for a specific one
k0s controller --feature-gates="AllAlpha=true,ExampleAlphaFeature=false"

# Enable all beta features and also enable a specific alpha feature
k0s controller --feature-gates="AllBeta=true,ExampleAlphaFeature=true"
```

## Available Features

The following table is a summary of the feature gates that you can set. Each feature is designed for a specific stage of stability:

| Feature           | Default | Stage | Since | Until |
|-------------------|---------|-------|-------|-------|
| `IPv6SingleStack` | `false` | Alpha | 1.34  |       |

### Feature Details

- `IPv6SingleStack`: Enables single-stack IPv6 support in k0s clusters.

## Lifecycle Management

Features progress through the following stages:

1. **Alpha**: Experimental, disabled by default, may change or be removed
2. **Beta**: More stable, still disabled by default, less likely to change
3. **GA**: Stable, enabled by default, feature gate eventually removed

Once a feature reaches GA status, the feature gate cannot be disabled anymore
and it will be eventually removed.
