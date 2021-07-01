# Airgap install

You can install k0s in an environment with restricted Internet access. Airgap installation requires an image bundle, which contains all the needed container images. There are two options to get the image bundle:

- Use a ready-made image bundle, which is created for each k0s release. It can be downloaded from the [releases page](https://github.com/k0sproject/k0s/releases/latest).
- Create your own image bundle. In this case, you can easily customize the bundle to also include container images, which are not used by default in k0s.

## Prerequisites

In order to create your own image bundle, you need

- A working cluster with at least one controller, to be used to build the image bundle. For more information, refer to the [Quick Start Guide](install.md).
- The containerd CLI management tool `ctr`, installed on the worker machine (refer to the ContainerD [getting-started](https://containerd.io/docs/getting-started/) guide).

## 1. Create your own image bundle (optional)

k0s/containerd uses OCI (Open Container Initiative) bundles for airgap installation. OCI bundles must be uncompressed. As OCI bundles are built specifically for each architecture, create an OCI bundle that uses the same processor architecture (x86-64, ARM64, ARMv7) as on the target system.

k0s offers two methods for creating OCI bundles, one using Docker and the other using a previously set up k0s worker. Be aware, though, that you cannot use the Docker method for the ARM architectures due to [kube-proxy image multiarch manifest problem](https://github.com/kubernetes/kubernetes/issues/98229).

### Docker

1. Pull the images.

   ```shell
   k0s airgap list-images | xargs -I{} docker pull {}
   ```

2. Create a bundle.

   ```shell
   docker image save $(k0s airgap list-images | xargs) -o bundle_file
   ```

### Previously set up k0s worker

As containerd pulls all the images during the k0s worker normal bootstrap, you can use it to build the OCI bundle with images.

Use the following commands on a machine with an installed k0s worker:

```shell
ctr --namespace k8s.io \
    --address /run/k0s/containerd.sock \
    images export bundle_file $(k0s airgap list-images | xargs)
```

## 2a. Sync the bundle file with the airgapped machine (locally)

Copy the `bundle_file` you created in the previous step or downloaded from the [releases page](https://github.com/k0sproject/k0s/releases/latest) to the target machine into the `images` directory in the k0s data directory. Copy the bundle only to the worker nodes. Controller nodes don't use it.

```shell
# mkdir -p /var/lib/k0s/images
# cp bundle_file /var/lib/k0s/images/bundle_file
```

## 2b. Sync the bundle file with the airgapped machines (remotely with k0sctl)

As an alternative to the previous step, you can use k0sctl to upload the bundle file to the worker nodes. k0sctl can also be used to upload k0s binary file to all nodes. Take a look at this example (k0sctl.yaml) with one controller and one worker node to upload the bundle file and k0s binary:

```YAML
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s-cluster
spec:
  hosts:
  - ssh:
      address: <ip-address-controller>
      user: ubuntu
      keyPath: /path/.ssh/id_rsa
    role: controller
    uploadBinary: true
    k0sBinaryPath: /path/to/k0s_binary/k0s
  - ssh:
      address: <ip-address-worker>
      user: ubuntu
      keyPath: /path/.ssh/id_rsa
    role: worker
    uploadBinary: true
    k0sBinaryPath: /path/to/k0s_binary/k0s
    files:
      - name: bundle-file
        src: /path/to/bundle-file/airgap-bundle-amd64.tar
        dstDir: /var/lib/k0s/images/
        perm: 0755
  k0s:
    version: 1.21.2+k0s.0
```

## 3. Ensure pull policy in the k0s.yaml (optional)

Use the following `k0s.yaml` to ensure that containerd does not pull images for k0s components from the Internet at any time.

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s
spec:
  images:
    default_pull_policy: Never
```

## 4. Set up the controller and worker nodes

Refer to the [Manual Install](k0s-multi-node.md) for information on setting up the controller and worker nodes locally. Alternatively, you can use [k0sctl](k0sctl-install.md).

**Note**: During the worker start up k0s imports all bundles from the `$K0S_DATA_DIR/images` before starting `kubelet`.
