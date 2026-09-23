<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Kubernetes AI conformance

This document summarizes how k0s satisfies the [Kubernetes AI Conformance](https://github.com/cncf/k8s-ai-conformance) requirements.
It is the reviewer-facing evidence page for the k0s submissions, one section per requirement, with the anchors referenced from `PRODUCT.yaml`.

| Kubernetes | k0s | Submission | Evidence |
| --- | --- | --- | --- |
| v1.36 | v1.36.4+k0s.1 | [cncf/k8s-ai-conformance v1.36/k0s](https://github.com/cncf/k8s-ai-conformance/tree/main/v1.36/k0s) | Automated test suite run plus the documented demonstrations below |
| v1.35 | v1.35.3+k0s.0 | [cncf/k8s-ai-conformance v1.35/k0s](https://github.com/cncf/k8s-ai-conformance/tree/main/v1.35/k0s) | Documented demonstrations below |

Two kinds of evidence appear on this page:

- **Automated verification.** Requirements covered by the upstream [AI Conformance test suite](https://github.com/kubernetes-sigs/ai-conformance/tree/main/test) are verified by running that suite against a freshly provisioned k0s cluster. The pipeline, its results and the exact versions are described under [Automated verification](#automated-verification). The test artifacts (`junit.xml`, `e2e.log`, `results.json`) are part of the CNCF submission.
- **Documented demonstrations.** Requirements without an automated test are verified by the captured demonstrations in each section. These were run on k0s v1.35.3+k0s.0; the requirement text for them is unchanged between the v1.35 and v1.36 checklists, and the components involved are installed the same way on both versions.

## Automated verification

The [kubernetes-sigs/ai-conformance](https://github.com/kubernetes-sigs/ai-conformance) suite is run by the [Mirantis/cncf-conformance](https://github.com/Mirantis/cncf-conformance) pipeline.
A run provisions a two-node k0s cluster on Azure with k0sctl, installs the NVIDIA GPU Operator and Kueue, runs the suite, uploads the artifacts and destroys the cluster.
Runs happen weekly against k0s `main` and on demand against a release; the submission uses a release run.

Run used for the v1.36 submission: [Mirantis/cncf-conformance run 35737735835](https://github.com/Mirantis/cncf-conformance/actions/runs/35737735835).

| Item | Value |
| --- | --- |
| k0s | v1.36.4+k0s.1 (Kubernetes v1.36.4, containerd 2.3.5) |
| Nodes | one `Standard_D2s_v3` controller, one `Standard_NC4as_T4_v3` worker (NVIDIA T4), Ubuntu 24.04 |
| Accelerator stack | NVIDIA GPU Operator v26.7.0, driver 595.91.07, device plugin allocation mode |
| Gang scheduler | Kueue v0.18.2 (`ResourceFlavor`, `ClusterQueue`, `LocalQueue`) |
| Suite | kubernetes-sigs/ai-conformance commit `da952539a75be3ceee8bbf82698c5f19c401cb2d` |
| Result | 126 tests, 0 failures, 1 skipped |

```console
$ go run gotest.tools/gotestsum@v1.13.0 --junitfile junit.xml --jsonfile results.json -- \
    ./test -v -timeout 30m -kubeconfig "$KUBECONFIG" -accelerator-type=nvidia -allocation-mode=auto \
    -gang-scheduler-namespace=ai-conformance-gang-scheduling -gang-job-labels=kueue.x-k8s.io/queue-name=e2e-lq
--- PASS: TestSecureAcceleratorAccess (125.39s)
--- PASS: TestGangScheduling (45.75s)
--- SKIP: TestAcceleratorClusterAutoscaling (0.00s)
DONE 126 tests, 1 skipped in 173.872s
```

| Test | Requirement | Outcome |
| --- | --- | --- |
| `TestSecureAcceleratorAccess` | [Secure accelerator access](#secure-accelerator-access) | Passed |
| `TestGangScheduling` | [Gang scheduling](#gang-scheduling) | Passed |
| `TestAcceleratorClusterAutoscaling` | [Cluster autoscaling](#cluster-autoscaling) | Skipped: the test needs an autoscaled accelerator node pool, which the test cluster does not have. k0s does not bundle a cluster autoscaler; see the section for how it integrates with the upstream one |

The remaining tests in the suite are unit tests of the test harness itself and do not depend on the cluster.

## Test environment

Automated verification ran on the cluster described in the table above.

The documented demonstrations were captured on a two-node k0s cluster running on Azure:

- k0s v1.35.3+k0s.0
- Kubernetes v1.35.3
- One controller node and one NVIDIA T4 worker node
- GPU Operator v26.3.1 for NVIDIA runtime, device plugin, and DCGM Exporter

## DRA support

**Requirement (v1.35 checklist only):** Dynamic Resource Allocation APIs must be available and usable for accelerator allocation.
This requirement was removed from the v1.36 checklist; the section is kept for the v1.35 submission.

**Status on k0s:** Implemented.

k0s v1.35.3+k0s.0 ships Kubernetes v1.35.3, where DRA is available at `resource.k8s.io/v1` and enabled by default.
We validated the full DRA flow with the upstream `dra-example-driver`: a `ResourceClaimTemplate` was created, the scheduler allocated a simulated GPU device, and the consuming Pod received the allocated device metadata inside the container.
This shows not only that the API surface exists, but that the allocation path is functional end to end on k0s.

Minimal example:

```console
$ kubectl api-resources --api-group=resource.k8s.io
NAME                     APIVERSION           KIND
deviceclasses            resource.k8s.io/v1   DeviceClass
resourceclaims           resource.k8s.io/v1   ResourceClaim
resourceclaimtemplates   resource.k8s.io/v1   ResourceClaimTemplate
resourceslices           resource.k8s.io/v1   ResourceSlice

$ kubectl -n dra-demo get resourceclaim
NAME                     STATE
dra-demo-pod-gpu-nd7zh   allocated,reserved

$ kubectl logs -n dra-demo dra-demo-pod | grep GPU_DEVICE_0
GPU_DEVICE_0="gpu-0"
```

Further reading:

- [Dynamic Resource Allocation in Kubernetes](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/)
- [kubernetes-sigs/dra-example-driver](https://github.com/kubernetes-sigs/dra-example-driver)

## Driver runtime management

**Requirement:** The platform should support installation and management of accelerator drivers and runtime components.

**Status on k0s:** Implemented.

k0s supports NVIDIA driver and runtime management through the NVIDIA GPU Operator.
In the automated run, GPU Operator v26.7.0 installed driver 595.91.07 on the k0s v1.36.4+k0s.1 worker, configured the `nvidia` runtime in k0s's containerd through the `/etc/k0s/containerd.d/` drop-in directory, and the operator's validator confirmed a working CUDA workload before the suite started.
On the demonstration cluster, the operator installed the NVIDIA driver, configured the NVIDIA runtime for containerd, and enabled GPU access for workloads that opt into the `nvidia` `RuntimeClass`.
This is the standard upstream integration path for NVIDIA-backed Kubernetes clusters and works on k0s with containerd runtime configuration.

Minimal example:

```console
$ kubectl -n gpu-operator exec ds/nvidia-driver-daemonset -- \
    nvidia-smi --query-gpu=driver_version --format=csv,noheader
580.126.20

$ kubectl logs cuda-smoketest
NVIDIA-SMI 580.126.20             Driver Version: 580.126.20

$ kubectl get runtimeclass
NAME           HANDLER
nvidia         nvidia
nvidia-cdi     nvidia-cdi
```

Further reading:

- [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/)
- [Using `nvidia-container-runtime` with k0s](https://docs.k0sproject.io/stable/runtime/#using-nvidia-container-runtime)

## GPU sharing

**Requirement:** The platform should support mechanisms to share a single physical accelerator among multiple workloads.

**Status on k0s:** Implemented.

k0s supports GPU sharing through NVIDIA device plugin time-slicing.
On the demonstration cluster, a single Tesla T4 was reconfigured to advertise four schedulable `nvidia.com/gpu` replicas, and four GPU-requesting Pods ran concurrently on the same physical GPU.
T4 hardware does not support MIG, so time-slicing is the applicable sharing mechanism on this setup.
This demonstrates a real sharing mode on hardware commonly used for inference and smaller training workloads.

Minimal example:

```console
$ kubectl get node k0s-gpu-0 -o jsonpath='{.status.capacity.nvidia\.com/gpu}{"\n"}'
4

$ kubectl get pods -l app=shared-gpu -o wide
NAME           READY   STATUS    NODE
shared-gpu-1   1/1     Running   k0s-gpu-0
shared-gpu-2   1/1     Running   k0s-gpu-0
shared-gpu-3   1/1     Running   k0s-gpu-0
shared-gpu-4   1/1     Running   k0s-gpu-0

$ kubectl get node k0s-gpu-0 -o jsonpath='{.metadata.labels.nvidia\.com/gpu\.sharing-strategy}{"\n"}'
time-slicing
```

Further reading:

- [NVIDIA k8s-device-plugin](https://github.com/NVIDIA/k8s-device-plugin)
- [NVIDIA GPU Operator time-slicing documentation](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/gpu-sharing.html)

## Virtualized accelerator

**Requirement:** The platform should support virtualized accelerators where available.

**Status on k0s:** Not Implemented.

k0s does not ship a packaged vGPU integration.
Virtualized GPU support depends on NVIDIA vGPU licensing and hardware-specific enablement outside the scope of the k0s distribution.
Users can integrate upstream NVIDIA vGPU components on suitable infrastructure, but that path was not demonstrated in this submission.

Further reading:

- [NVIDIA vGPU documentation](https://docs.nvidia.com/grid/)
- [NVIDIA GPU Operator](https://docs.nvidia.com/datacenter/cloud-native/gpu-operator/latest/)

## AI inference

**Requirement:** A working Kubernetes Gateway API implementation must support inference traffic management.

**Status on k0s:** Implemented.

k0s supports Gateway API resources and standard Gateway implementations.
We installed Gateway API v1.5.1 CRDs with Envoy Gateway v1.7.2, then configured a `GatewayClass`, `Gateway`, and `HTTPRoute` to route `/predict` traffic to a mock inference backend.
Requests matching the configured prefix reached the backend, while non-matching requests were rejected by Envoy.
This provides concrete evidence that k0s can host an inference ingress stack with route attachment and path-based traffic management.

Minimal example:

```console
$ kubectl get gatewayclass eg
NAME   CONTROLLER                                      ACCEPTED
eg     gateway.envoyproxy.io/gatewayclass-controller   True

$ curl -si http://localhost:8888/predict
HTTP/1.1 200 OK
x-app-name: http-echo
{"prediction":0.87}

$ curl -si http://localhost:8888/
HTTP/1.1 404 Not Found
```

Further reading:

- [Gateway API](https://gateway-api.sigs.k8s.io/)
- [Envoy Gateway](https://gateway.envoyproxy.io/)

## Advanced inference ingress

**Requirement:** The platform should support an implementation of the Gateway API Inference Extension for model-aware routing.

**Status on k0s:** Not Implemented.

k0s does not bundle a Gateway API Inference Extension implementation.
Gateway implementations that support the Inference Extension can be installed on k0s like any other Gateway API implementation, but that path was not demonstrated in this submission.

Further reading:

- [Gateway API Inference Extension](https://gateway-api-inference-extension.sigs.k8s.io/)

## High performance networking

**Requirement:** If high performance pod-to-pod communication is needed, the platform should expose specialized network resources, preferably through DRA.

**Status on k0s:** Not Implemented.

k0s does not bundle RDMA, GPUDirect or multi-network components.
These depend on host hardware and drivers outside the distribution and were not demonstrated in this submission.

Further reading:

- [DRANET](https://github.com/kubernetes-sigs/dranet)

## Disaggregated inference

**Requirement:** The platform should be able to install and run a disaggregated inference solution with separately scalable prefill and decode phases.

**Status on k0s:** Not Implemented.

k0s does not bundle a disaggregated inference stack.
Solutions such as llm-d or Dynamo can be installed on k0s as ordinary workloads, but that path was not demonstrated in this submission.

Further reading:

- [llm-d](https://llm-d.ai/)

## Gang scheduling

**Requirement:** The platform must support all-or-nothing scheduling for distributed workloads.

**Status on k0s:** Implemented.

Automated verification: `TestGangScheduling` from the upstream suite passed on k0s v1.36.4+k0s.1 with Kueue v0.18.2 as the gang scheduler.
The test submits a two-pod Job that fits and checks that both pods are bound together and complete, then submits a Job that cannot fit and checks that it stays suspended with zero bound pods for the whole observation window.

```console
=== RUN   TestGangScheduling/PositiveGangScheduling
    gang_scheduling_test.go:203:   Pod pos-job-c7ggp: schedulerName=default-scheduler, nodeName=k0s-gpu-0, phase=Succeeded
    gang_scheduling_test.go:203:   Pod pos-job-kvpzm: schedulerName=default-scheduler, nodeName=k0s-gpu-0, phase=Succeeded
    gang_scheduling_test.go:207: Positive job pos-job completed with 2 pods verified
=== RUN   TestGangScheduling/NegativeGangScheduling
    gang_scheduling_test.go:245: Negative job neg-job successfully remained suspended or pending with 0 bound pods throughout the 30s verification window.
--- PASS: TestGangScheduling (45.75s)
```

Documented demonstration: k0s also supports gang scheduling with Volcano.
We installed Volcano v1.14.1 and demonstrated both successful atomic placement and full refusal of an oversized gang: a two-task job was bound together, while a five-task job that could not fit was kept pending without partial placement.

```console
$ kubectl get vcjob gang-demo
NAME        STATUS      MINAVAILABLE
gang-demo   Completed   2

$ kubectl get vcjob gang-overflow
NAME            STATUS    MINAVAILABLE
gang-overflow   Pending   5

$ kubectl get pods -l volcano.sh/job-name=gang-overflow
No resources found in default namespace.
```

Further reading:

- [Kueue](https://kueue.sigs.k8s.io/)
- [Volcano](https://volcano.sh/en/)

## Cluster autoscaling

**Requirement:** The platform must support scaling accelerator-capable node groups through Kubernetes cluster autoscaling.

**Status on k0s:** Implemented.

k0s is a Kubernetes distribution and does not bundle its own autoscaler.
Instead, k0s works with the upstream Kubernetes Cluster Autoscaler and the infrastructure-specific provisioning layer behind the cluster, such as Azure VMSS, AWS Auto Scaling Groups, GCP Managed Instance Groups, or Cluster API.
Accelerator-aware scale-up is driven by pending Pods that request `nvidia.com/gpu` and by the GPU resources advertised through the NVIDIA device plugin.
This is the same model used by other conformant distributions: k0s provides the standard Kubernetes substrate, while autoscaling is supplied by the provisioning layer chosen for the cluster.

The upstream suite's `TestAcceleratorClusterAutoscaling` is skipped in the automated run because the fixed two-node test cluster has no autoscaled node pool to exercise.

Reference configuration:

```shell
helm repo add autoscaler https://kubernetes.github.io/autoscaler
helm install cluster-autoscaler autoscaler/cluster-autoscaler \
  --namespace kube-system \
  --set cloudProvider=azure
```

Further reading:

- [Kubernetes Cluster Autoscaler](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler)
- [Cluster Autoscaler GPU support](https://github.com/kubernetes/autoscaler/tree/master/cluster-autoscaler#gpu-support)

## Pod autoscaling

**Requirement:** Horizontal Pod Autoscaler must function correctly for Pods that use accelerators.

**Status on k0s:** Implemented.

k0s bundles `metrics-server`, so HPA works out of the box on a default cluster.
We deployed a GPU-requesting `Deployment` and an `autoscaling/v2` HPA targeting CPU utilization.
Under load, HPA increased the Deployment replica count from one to two; the second Pod remained pending because the scheduler correctly enforced single-GPU node capacity.
This is the expected and correct interaction: autoscaling decisions are made from workload metrics, while accelerator capacity is still enforced independently by the scheduler.

Minimal example:

```console
$ kubectl get hpa gpu-burner
NAME         REFERENCE               TARGETS         MINPODS   MAXPODS   REPLICAS
gpu-burner   Deployment/gpu-burner   cpu: 497%/50%   1         2         2

$ kubectl get pods -l app=gpu-burner
NAME                          READY   STATUS
gpu-burner-759c8596bf-7gzvr   1/1     Running
gpu-burner-759c8596bf-k8d4x   0/1     Pending
```

Further reading:

- [Horizontal Pod Autoscaling](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [metrics-server](https://github.com/kubernetes-sigs/metrics-server)

## Accelerator metrics

**Requirement:** The platform must expose fine-grained accelerator metrics through a standard, machine-readable endpoint.

**Status on k0s:** Implemented.

k0s supports accelerator metrics through NVIDIA DCGM Exporter, deployed by the GPU Operator.
We integrated DCGM Exporter with `kube-prometheus-stack` using a `ServiceMonitor` and verified that Prometheus scraped per-GPU metrics with labels such as UUID, model, PCI bus ID, node name, and driver version.
The important part here is not just that metrics exist, but that they are queryable through the standard Prometheus API with enough labels to identify a specific GPU on a specific node.

Minimal example:

```console
$ curl -s 'http://localhost:9090/api/v1/query?query=DCGM_FI_DEV_GPU_UTIL'
"__name__":"DCGM_FI_DEV_GPU_UTIL"
"UUID":"GPU-df2bfa0a-183e-d378-e4ab-1042a1736a51"
"modelName":"Tesla T4"
"Hostname":"k0s-gpu-0"
"pci_bus_id":"00000001:00:00.0"
```

Further reading:

- [NVIDIA DCGM Exporter](https://docs.nvidia.com/datacenter/dcgm/latest/gpu-telemetry/dcgm-exporter.html)
- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)

## AI service metrics

**Requirement:** The platform must discover and collect workload metrics in a standard format.

**Status on k0s:** Implemented.

k0s supports workload metrics collection through Prometheus Operator compatible monitoring stacks.
We installed `kube-prometheus-stack`, created a `ServiceMonitor` for a sample inference-shaped workload, and verified that Prometheus discovered the target and stored its metrics with workload-identifying labels.
This demonstrates the standard collection path most AI services use in practice: workload metrics exposed on `/metrics`, discovered by `ServiceMonitor`, and stored in Prometheus.

Minimal example:

```console
$ curl -s 'http://localhost:9090/api/v1/query' \
    --data-urlencode 'query=up{job="sample-inference"}'
"job":"sample-inference"
"namespace":"default"
"service":"sample-inference"
"value":[..., "1"]

$ curl -s 'http://localhost:9090/api/v1/query' \
    --data-urlencode 'query=node_cpu_seconds_total{job="sample-inference"}'
"__name__":"node_cpu_seconds_total"
```

Further reading:

- [Prometheus Operator `ServiceMonitor`](https://prometheus-operator.dev/docs/developer/getting-started/#using-servicemonitors)
- [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)

## Secure accelerator access

**Requirement:** Accelerator access must be isolated so that only authorized workloads can use the device.

**Status on k0s:** Implemented.

Automated verification: `TestSecureAcceleratorAccess` from the upstream suite passed on k0s v1.36.4+k0s.1 with the GPU Operator device plugin.
The test runs a probe in three shapes: a Pod that requests one `nvidia.com/gpu` and must see exactly one device, a Pod without a request that must see none, and a two-container Pod where only the requesting container may see the device.

```console
=== RUN   TestSecureAcceleratorAccess
    util_test.go:273: Auto-detected allocation mode: device-plugin
    util_test.go:859: PASS: prober sees exactly 1 accelerator device(s).
    util_test.go:859: PASS: prober sees exactly 0 accelerator device(s).
    util_test.go:859: PASS: authorized sees exactly 1 accelerator device(s).
    util_test.go:859: PASS: unauthorized sees exactly 0 accelerator device(s).
--- PASS: TestSecureAcceleratorAccess (125.39s)
```

Documented demonstration: on the demonstration cluster, a Pod without a GPU request and without the `nvidia` `RuntimeClass` had no access to NVIDIA tooling or device nodes, and when two Pods requested one GPU each on a single-GPU node, the scheduler admitted one and kept the second pending with `Insufficient nvidia.com/gpu`.

```console
$ kubectl logs no-gpu-request
sh: 1: nvidia-smi: not found
NO_GPU_ACCESS

$ kubectl get pods gpu-hog-1 gpu-hog-2
NAME        READY   STATUS
gpu-hog-1   1/1     Running
gpu-hog-2   0/1     Pending
```

Further reading:

- [Kubernetes device plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/)
- [NVIDIA k8s-device-plugin](https://github.com/NVIDIA/k8s-device-plugin)

## Robust controller

**Requirement:** The platform must be able to run at least one complex AI operator with CRDs and reliable reconciliation behavior.

**Status on k0s:** Implemented.

k0s supports complex AI operators such as KubeRay.
We installed KubeRay v1.6.1, reconciled a `RayCluster` into head and worker Pods plus supporting Services, ran a distributed Ray task successfully, and then deleted the head Pod to verify that the operator recreated it and restored the cluster to a ready state.
This goes beyond a basic install check: it shows CRD registration, reconciliation of child resources, a successful workload, and recovery after disruption.

Minimal example:

```console
$ kubectl get raycluster raycluster-demo -o wide
NAME              DESIRED WORKERS   AVAILABLE WORKERS   STATUS
raycluster-demo   1                 1                   ready

$ kubectl get events --field-selector involvedObject.kind=RayCluster \
    --sort-by=.lastTimestamp
CreatedHeadPod
CreatedWorkerPod
DeletedHeadPod
CreatedHeadPod

$ kubectl exec -i "$HEAD_POD" -- python -
Result: [0, 1, 4, 9, 16]
Cluster resources: {'CPU': 2.0, ...}
```

Further reading:

- [KubeRay](https://docs.ray.io/en/latest/cluster/kubernetes/)
