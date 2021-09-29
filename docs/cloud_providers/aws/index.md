# cloud-provider-aws

The AWS cloud provider provides the interface between a
Kubernetes cluster and AWS service APIs.
This project allows a Kubernetes cluster to provision,
monitor and remove AWS resources necessary for operation of the cluster.

[See the official online documentation here](https://cloud-provider-aws.sigs.k8s.io)

## k0s specific instructions

**Before doing steps above, follow the [official prerequisites guide](https://cloud-provider-aws.sigs.k8s.io/prerequisites/)**

## Cluster installation

### k0sctl installation

Use following k0sctl config as a starting point

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
spec:
  hosts:
  - role: controller
    ssh:
      address: 10.0.0.1 # replace with the controller's IP address
      user: root
      keyPath: ~/.ssh/id_rsa
  - role: worker
    ssh:
      address: 10.0.0.2 # replace with the worker's IP address
      user: root
      keyPath: ~/.ssh/id_rsa
      installFlags:
        - --enable-cloud-provider
        - --labels node-role.kubernetes.io/master=""
```

### Manual install

#### Controller install

Follow the [guide for controller installation](../install.md)

#### Worker install

It is required to pass `--enbable-cloud-provider` argument to the k0s worker install command.
It is allowed to use any other arguments as usual

```shell
k0s install worker --enable-cloud-worker --token-file <token_filepath> -с <path-to-k0s-worker-config> --labels node-role.kubernetes.io/master=""
```

### Notes on the worker node

By default, k0s has separated workers from the control plane. From the other side, usually a node that runs control plane has special label node-role/master. That effectively means in k0s cluster by default no master worker node. While it is perfectly fine for almost any type of workload, some lower level components use that label as a node selector.
That is why we manually mark kubelet with label in both examples, manual one and k0sctl driven.

### Notes on ready status after cluster installation

Because in current set up we use external cloud providers, any worker node joined cluster will be marked as NonReady by using taint "node.cloudprovider.kubernetes.io/uninitialized". That is absolutely expected behavior until we install a cloud provider.

## Finishing cloud provider installation

From that moment you should follow [official installation guide](https://cloud-provider-aws.sigs.k8s.io/getting_started/)

## Useful links

[Working with IAM policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies_create.html)

[Amazon Resource Name](https://docs.aws.amazon.com/general/latest/gr/aws-arns-and-namespaces.html)

[Taints and tolerations](https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/)
