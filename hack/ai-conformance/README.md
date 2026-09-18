<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Running the CNCF AI Conformance suite

Runs the [Kubernetes AI Conformance test suite][suite] against a fresh k0s
cluster with one NVIDIA T4 worker on Azure and collects the artifacts for a
[CNCF AI Conformance submission][cncf].

[suite]: https://github.com/kubernetes-sigs/ai-conformance
[cncf]: https://github.com/cncf/k8s-ai-conformance/blob/main/instructions.md

## What it does

| Stage | Tool |
| --- | --- |
| Resource group, VNet, NSG, two VMs, pod-CIDR route table | `az` |
| k0s install, controller+worker and one GPU worker | `k0sctl` |
| NVIDIA GPU Operator, wait for `nvidia.com/gpu: 1` | `helm` |
| Kueue plus ResourceFlavor, ClusterQueue, LocalQueue | `kubectl` |
| Test suite | `go` via gotestsum |
| Delete the resource group | `az`, on every exit path |

`TestAcceleratorClusterAutoscaling` is skipped on purpose: k0s ships no
cluster autoscaler. Gang scheduling is tested with Kueue because the suite
submits a plain `batch/v1` Job that Volcano does not intercept.

## Requirements

`az`, `k0sctl`, `kubectl`, `helm`, `go` (1.25 or newer, per the suite's
`go.mod`), `jq`, `git`, `ssh`, `ssh-keygen`, `ssh-keyscan`.

## Usage

```shell
hack/ai-conformance/run.sh --suite-sha da952539a75b
```

This tests the current stable k0s release; pass `--k0s-version` to test
another one. See `run.sh --help` for the other options and environment
variables. The SSH key pair is generated per run and discarded with the
cluster.

## Output

`artifacts/<timestamp>/`:

| Path | Purpose |
| --- | --- |
| `submission/` | `junit.xml`, `e2e.log`, `results.json` in the format the CNCF validator expects; copied verbatim into `v1.3x/k0s/` |
| `run-metadata.json` | Versions, region, VM sizes, per-stage timings; source for the PRODUCT.yaml `notes` |
| `debug/` | `timings.csv`, `k0sctl.yaml`, `k0sctl.log`, `helm-values/`, and `cluster-state/` dumps taken before the VMs are deleted |

Exit status 0 means every test passed or was skipped. On any other exit
`submission/` is renamed to `failed/` so it cannot be submitted by accident.
The resource group is deleted on every exit path unless `--keep-cluster` is
given, in which case `kubeconfig` and the SSH key stay in the artifact
directory.

## CI

`.github/workflows/ai-conformance.yaml` runs this script every Monday and on
`workflow_dispatch`. It authenticates to Azure with an OIDC federated
credential on a service principal that has Contributor on the target
subscription, so the repository only stores three non-secret IDs:
`AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`. The artifact
directory is uploaded on every run, minus `kubeconfig` and the SSH key, and a
second `az group delete` runs as a safety net.

## k0s specifics

* k0s runs its own containerd at `/run/k0s/containerd.sock` and imports
  drop-ins from `/etc/k0s/containerd.d/`. The GPU Operator toolkit is pointed
  there via `gpu-operator-values.yaml`; k0s notices the new drop-in and
  restarts containerd by itself.
* Azure drops pod traffic between nodes unless IP forwarding is enabled on the
  NICs and a route table sends each node's pod CIDR to that node. The script
  does both and re-checks the CIDRs after k0s has assigned them.
* The NVIDIA driver container builds against the running kernel, so the
  matching `linux-headers` package is installed on the worker before the
  operator starts.
