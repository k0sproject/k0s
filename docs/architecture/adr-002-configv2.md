# ADR 2: Configuration v2

## Overview

This ADR is very high level proposal for the overall goals and high level aspirations for configuration v2. As such the puspose is not to go overly deep into where/how different fields are defined.

The examples provided in this ADR are currently not exhaustive. So do NOT pay too much attention to each field, they're meant to be more examples of the _types_ of configuration for each object.

## Context

The configration of k0s has organically growed from very small to something quite complex today. While that is natural we're starting to see growing number of issues stemmin from the complexity.

- Lack of separation between per node configuration and cluster configuration
- Lack of feedback in dynamic config reconciliation status
- Lack of feedback in helm and stack applier status (should these be in the clusterconfig to begin with?)
- No separation between used defined configuration and the configuration that is used internally
- Lack of versioning
- Dynamic config has several problems when it comes to reconciliation of certain fields
- Some fields have side effects that aren't very predictable.
- The k0s' configuration for calico doesn't really match the calico configuration
- Config doesn't allow to disable components, it requires modifying the cmdline, specially with dynamic config, it would make sense to allow this from the configuration.
- In dynamicConfig it's not very straightforward what can and what cannot be modified

All of this results in confusions on both the uers side and also on maintainers.

## Config v2 goals

The maintainers have agreed on set of high level goals for config v2:

### Config as Kubernetes objects

Configs shall be formatted as Kubernetes object, even if we do not really store them on API. K8s object are the natural language for k0s users and it keeps the option open to actually store everything in the API.

### Per node and cluster wide config separation

Separate the per node configuration and cluster-wide configs into their own CRDs. This makes it clear for both the users and maintainers where to look for which config data. It also makes clear separation on which _things_ can be changed at runtime via dynamic config.

As an example, we would have `ControllerConfig` and `ClusterConfig` for the controlplanes:

```yaml
# ControllerConfig contains only the node specific bits for a controller node
# So basically only the bits that we need to boot up etcd/kine and the API server
apiVersion: k0sproject.io/...
name: k0s
kind: ControllerConfig
spec:
  etcd:
    privateAddress: 1.2.3.4
  api:
    listenAddress: 1.2.3.4:6443,[::0]:6443
    address: foobar.com:6443
    sans:
      - foobar.com
  enableWorker: true
  disableComponents: ["foo", "bar"]
status:
  lastReconciledConfig: "<ClusterConfig version: "Full object">"
  history: # some kind of history when was which config applied etc.
---
# ClusterConfig contains all the bits that are always cluster-wide
apiVersion: k0sproject.io/...
name: k0s
kind: ClusterConfig
spec:
  network:
    provider: calico
    calico:
      foo: bar
```

Similarly we need to have config object for the workers. An example:

```yaml
apiVersion: k0sproject.io/...
name: k0s
kind: WorkerConfig
spec:
  kubelet:
    args:
      foo: bar
  containerd:
    args:
      foo: bar
  profile: foobar
```

Let's not break the configs into many separate CRD's, instead use more monolithic approach. One of the deciding factors is whether there's value in highly separated status information. If so, it might warrant a dedicated CRD.

## Versioning

It's clear we need to support both v1 (current) and v2 way of configuration. Also looking forward, we need to ensure that we can move beyond v2 in controlled fashion. Essentially this means that we need to have transformers in place to allow reading in v1 config but internally translating that into v2. And in future from v2 to v3 and so on.

We should utilize the well-known pattern for CRD versioning as outlined in https://book.kubebuilder.io/multiversion-tutorial/conversion-concepts.

## Default values

For config v2, we can change some of the defaults too. Currently we're "stuck" in some cases for non-optimal defaults. But since starting to use config v2 is a concious decision for the users we can change some of the defaults too. As an example, we could change the pod/container log paths to be within the k0s data directory. (`/var/lib/k0s`)

## Status

Proposed.

This has been quite extensively discussed among core maintainers and thus these high level goals and aspirations are well aligned already.

The plan is to enhance this proposal during the implementation phases. We need to have phased approach for the implementation anyways as this is very big topic and cutting through pretty much every piece of k0s.

## Consequences if not implemented

The confusions on both on users and on maintainers will continue, cause pain and bugs.
