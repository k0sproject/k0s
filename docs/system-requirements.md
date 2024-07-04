# System requirements

This page describes the system requirements for k0s.

## Minimum memory and CPU requirements

The minimum requirements for k0s detailed below are approximations, and thus your results may vary.

| Role                | Memory (RAM) | Virtual CPU (vCPU) |
|---------------------|--------------|--------------------|
| Controller node     | 1   GB       | 1 vCPU             |
| Worker node         | 0.5 GB       | 1 vCPU             |
| Controller + worker | 1   GB       | 1 vCPU             |

## Controller node recommendations

| # of Worker nodes | # of Pods    | Recommended RAM | Recommended vCPU |
|-------------------|--------------|-----------------|------------------|
| up to   10        | up to   1000 | 1-2   GB        | 1-2   vCPU       |
| up to   50        | up to   5000 | 2-4   GB        | 2-4   vCPU       |
| up to  100        | up to  10000 | 4-8   GB        | 2-4   vCPU       |
| up to  500        | up to  50000 | 8-16  GB        | 4-8   vCPU       |
| up to 1000        | up to 100000 | 16-32 GB        | 8-16  vCPU       |
| up to 5000        | up to 150000 | 32-64 GB        | 16-32 vCPU       |

k0s has the standard Kubernetes limits for the maximum number of nodes, pods, etc. For more details, see [the Kubernetes considerations for large clusters](https://kubernetes.io/docs/setup/best-practices/cluster-large/).

k0s controller node measured memory consumption can be found below on this page.

## Storage

It's recommended to use an SSD for [optimal storage performance](https://etcd.io/docs/current/op-guide/performance/) (cluster latency and throughput are sensitive to storage).

For worker nodes it is required that there is at least 15% relative disk space free.

The specific storage consumption for k0s is as follows:

| Role                 | Usage (k0s part) | Minimum required |
|----------------------|------------------|------------------|
| Controller node      | ~0.5 GB          | ~0.5 GB          |
| Worker node          | ~1.3 GB          | ~1.6 GB          |
| Controller + worker  | ~1.7 GB          | ~2.0 GB          |

**Note**: The operating system and application requirements must be considered in addition to the k0s part.

## Host operating system

- Linux (see [Linux specific requirements] for details)
- Windows Server 2019

[Linux specific requirements]: external-runtime-deps.md#linux-specific

## Architecture

- x86-64
- ARM64
- ARMv7

## Networking

For information on the required ports and protocols, refer to [networking](networking.md).

## External runtime dependencies

k0s strives to be as independent from the OS as possible. The current and past
external runtime dependencies are documented [here](external-runtime-deps.md).

To run some automated compatiblility checks on your system, use
[`k0s sysinfo`](cli/k0s_sysinfo.md).

## Controller node measured memory consumption

The following table shows the measured memory consumption in the cluster of one controller node.

| # of Worker nodes | # of Pods (besides default) | Memory consumption |
|-------------------|-----------------------------|--------------------|
| 1                 | 0                           | 510  MB            |
| 1                 | 100                         | 600  MB            |
| 20                | 0                           | 660  MB            |
| 20                | 2000                        | 1000 MB            |
| 50                | 0                           | 790  MB            |
| 50                | 5000                        | 1400 MB            |
| 100               | 0                           | 1000 MB            |
| 100               | 10000                       | 2300 MB            |
| 200               | 0                           | 1500 MB            |
| 200               | 20000                       | 3300 MB            |

Measurement details:

- k0s v1.22.4+k0s.2 (default configuration with etcd)
- Ubuntu Server 20.04.3 LTS, OS part of the used memory was around 180 MB
- Hardware: AWS t3.xlarge (4 vCPUs, 16 GB RAM)
- Pod image: nginx:1.21.4
