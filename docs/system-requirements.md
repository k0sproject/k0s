# System requirements

Verify that your environment meets the system requirements for k0s.

## Hardware

The minimum hardware requirements for k0s detailed below are approximations and
thus results may vary.

| Role                | Virtual CPU (vCPU)     | Memory (RAM)             |
|---------------------|------------------------|--------------------------|
| Controller node     | 1 vCPU (2 recommended) | 1 GB (2 recommended)     |
| Worker node         | 1 vCPU (2 recommended) | 0.5 GB (1 recommended)   |
| Controller + worker | 1 vCPU (2 recommended) | 1 GB (2 recommended)     |

**Note**: Use an SSD for [optimal storage performance](https://etcd.io/docs/current/op-guide/performance/) (cluster
latency and throughput are sensitive to storage).

The specific storage consumption for k0s is as follows:

| Role                 | Storage (k0s part) |
|----------------------|--------------------|
| Controller node      | ~0.5 GB            |
| Worker node          | ~1.3 GB            |
| Controller + worker  | ~1.7 GB            |

**Note**: The operating system and application requirements must be considered
in addition to the k0s part.

## Host operating system

- Linux (kernel v3.10 or later)
- Windows Server 2019

## Architecture

- x86-64
- ARM64
- ARMv7

## Networking

For information on the ports that k0s needs to function, refer to [networking](networking.md).

## OS dependencies

k0s is packaged as a fully statically linked binary which contains all other needed components also as statically linked binaries. This means that in typical use cases there's no OS level dependencies.

However some of the underlying components _may_ have dependencies to OS level tools and packages in certain circumstances. The known cases are documented in below.

### ContainerD needs `apparmor_parser`

If containerD [detects](https://github.com/containerd/containerd/blob/587fc092598791ab58bfa275958ce20cc5d80783/pkg/apparmor/apparmor_linux.go#L35-L44) that the system is configured to use [AppArmor](https://wiki.ubuntu.com/AppArmor) it will require a tool called `apparmor_parser` to be installed on the system.
