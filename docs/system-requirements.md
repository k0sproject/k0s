# System requirements

These are the k0s system requirements.

## Hardware

Minimum HW requirements.

| Role                | Virtual CPU (vCPU)     | Memory (RAM)           |
|---------------------|------------------------|------------------------|
| Controller node     | 1 vCPU (2 recommended) | 1 GB (2 recommended)   |
| Worker node         | 1 vCPU (2 recommended) | 1 GB (2 recommended)   |
| Controller + worker | 1 vCPU (2 recommended) | 1 GB (2 recommended)   |

For optimal storage performance we recommend using an SSD disk. Cluster latency and throughput are sensitive to storage:
[https://etcd.io/docs/current/op-guide/performance/](https://etcd.io/docs/current/op-guide/performance/)

k0s part of the storage consumption is presented in the following table. Note that the operating system and application requirements must be added on top.

| Role                 | Storage (k0s part) |
|----------------------|--------------------|
| Controller node      | ~0.5 GB            |
| Worker node          | ~1.3 GB            |
| Controller + worker  | ~1.7 GB            |

## Host operating system

- Linux (kernel v3.10 or newer)
- Windows Server 2019

## Architecture

- Intel (x86-64)
- ARM (ARM64)

## Networking

See [networking](networking.md) for the needed open ports.
