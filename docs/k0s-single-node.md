- [Creating a single-node cluster](#creating-a-single-node-cluster)
  - [Prerequisites](#prerequisites)
  - [Start k0s](#start-k0s)
      - [1. Generate a default cluster configuration](#1-generate-a-default-cluster-configuration)
      - [2. Start k0s](#2-start-k0s)
  - [Use k0s to access the cluster](#use-k0s-to-access-the-cluster)


# Creating a single-node cluster
These instructions outline a quick method for running a local k0s master and worker in a single node.

 **_NOTE:_**  This method of running k0s is only recommended for dev, test or POC environments.
 
## Prerequisites

Install k0s as documented in the [installation instructions](install.md).

## Start k0s
#### 1. Generate a default cluster configuration
```sh
k0s default-config > k0s.yaml
```

#### 2. Start k0s
```sh
$ sudo k0s install controller -c /full/path/to/k0s.yaml --enable-worker
INFO[2021-02-25 15:34:59] Installing k0s service
$ sudo systemctl start k0scontroller.service
```

## Use k0s to access the cluster

```sh
$ k0s kubectl get nodes
```
