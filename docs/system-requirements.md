<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# System requirements

This page describes the system requirements for k0s.

## Minimum memory and CPU requirements

The minimum requirements for k0s detailed below are approximations, and thus your results may vary.

| Role                | Memory (RAM in GB) | Virtual CPUs (vCPU) |
|---------------------|--------------------|---------------------|
| Controller node     |                  1 |                   1 |
| Worker node         |                0.5 |                   1 |
| Controller + worker |                  1 |                   1 |

## Controller node recommendations

| # of worker nodes | # of pods | Recommended RAM (in GB) | Recommended vCPU |
|-------------------|-----------|-------------------------|------------------|
|                10 |      1000 |                     1-2 |              1-2 |
|                50 |      5000 |                     2-4 |              2-4 |
|               100 |     10000 |                     4-8 |              2-4 |
|               500 |     50000 |                    8-16 |              4-8 |
|              1000 |    100000 |                   16-32 |             8-16 |
|              5000 |    150000 |                   32-64 |            16-32 |

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

k0s runs on Linux and Windows operating systems.

The following operating systems are automatically tested as part of our CI:

<!-- NOTE: Update ostests-nightly.yaml if you change the tested OS list here. -->

| Operating System              | Version/Notes                                                                           |
|-------------------------------|-----------------------------------------------------------------------------------------|
| Amazon Linux                  | 2023                                                                                    |
| Alpine Linux                  | 3.20, 3.23                                                                              |
| CentOS Stream                 | 9, 10 (Coughlan)                                                                        |
| Debian GNU/Linux              | 11 (bullseye) (supported until 2026-08-31), 12 (bookworm) (supported until 2028-06-30)  |
| Fedora CoreOS                 | stable stream                                                                           |
| Fedora Linux                  | 41 (Cloud Edition)                                                                      |
| Flatcar Container Linux       | by Kinvolk                                                                              |
| Red Hat Enterprise Linux      | 7.9 (Maipo), 8.10 (Ootpa), 9.7 (Plow)                                                   |
| Rocky Linux                   | 8.10 (Green Obsidian), 9.5 (Blue Onyx)                                                  |
| SUSE Linux Enterprise Server  | 15 SP6                                                                                  |
| Ubuntu                        | 20.04 LTS, 22.04 LTS, 24.04                                                             |

**Note:** For detailed Linux-specific requirements, please refer to the [Linux specific requirements].

[Linux specific requirements]: external-runtime-deps.md#linux-specific

## Architecture

- `x86_64`
- `aarch64`
- `armv7l`
- `riscv64` (No pre-compiled binaries, no CI coverage)

## Networking

For information on the required ports and protocols, refer to [networking](networking.md).

## External runtime dependencies

k0s strives to be as independent from the operating system as possible. See the
dedicated section on [external runtime dependencies](external-runtime-deps.md)
for details on current and past requirements.

To run some automated compatiblility checks on your system, use
[`k0s sysinfo`](cli/k0s_sysinfo.md).

## Controller node measured memory consumption

The following table shows the measured memory consumption in the cluster of one controller node.

| # of Worker nodes | # of Pods (besides default) | Memory consumption (in MB) |
|-------------------|-----------------------------|----------------------------|
|                 1 |                           0 |                        510 |
|                 1 |                         100 |                        600 |
|                20 |                           0 |                        660 |
|                20 |                        2000 |                       1000 |
|                50 |                           0 |                        790 |
|                50 |                        5000 |                       1400 |
|               100 |                           0 |                       1000 |
|               100 |                       10000 |                       2300 |
|               200 |                           0 |                       1500 |
|               200 |                       20000 |                       3300 |

Measurement details:

- k0s v1.22.4+k0s.2 (default configuration with etcd)
- Ubuntu Server 20.04.3 LTS, OS part of the used memory was around 180 MB
- Hardware: AWS t3.xlarge (4 vCPUs, 16 GB RAM)
- Pod image: nginx:1.21.4
