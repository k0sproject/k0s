# Airgap install

You can install k0s in an environment with restricted Internet access. Airgap installation requires an image bundle, which contains all the needed container images. There are two options to get the image bundle:

- Use a ready-made image bundle, which is created for each k0s release. It can be downloaded from the [releases page](https://github.com/k0sproject/k0s/releases/latest).
- Create your own image bundle. In this case, you can easily customize the bundle to also include container images, which are not used by default in k0s.

## Prerequisites

In order to create your own image bundle, you need:

- A working cluster with at least one controller that will be used to build the
  image bundle. See the [Quick Start Guide] for more information.
- The containerd CLI management tool `ctr`, installed on the worker node. See
  the [containerd Getting Started Guide] for more information.

[Quick Start Guide]: install.md
[containerd Getting Started Guide]: https://github.com/containerd/containerd/blob/v1.7.23/docs/getting-started.md

## 1. Create your own image bundle (optional)

k0s/containerd uses OCI (Open Container Initiative) bundles for airgap installation. OCI bundles must be uncompressed. As OCI bundles are built specifically for each architecture, create an OCI bundle that uses the same processor architecture (x86-64, ARM64, ARMv7) as on the target system.

k0s offers two methods for creating OCI bundles, one using Docker and the other using a previously set up k0s worker.

**Note:** When importing the image bundle k0s uses containerd "loose" [platform matching](https://pkg.go.dev/github.com/containerd/containerd/platforms#Only). For arm/v8, it will also match arm/v7, arm/v6 and arm/v5. This means that your bundle can contain multi arch images and the import will be done using platform compatibility.

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

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s-cluster
spec:
  k0s:
    version: {{{ extra.k8s_version }}}+k0s.0
  hosts:
    - role: controller
      ssh:
        address: <controller-ip-address>
        user: ubuntu
        keyPath: /path/.ssh/id_rsa

      #  uploadBinary: <boolean>
      #    When true the k0s binaries are cached and uploaded
      #    from the host running k0sctl instead of downloading
      #    directly to the target host.
      uploadBinary: true

      #  k0sBinaryPath: <local filepath>
      #    Upload a custom or manually downloaded k0s binary
      #    from a local path on the host running k0sctl to the
      #    target host.
      # k0sBinaryPath: path/to/k0s_binary/k0s

    - role: worker
      ssh:
        address: <worker-ip-address>
        user: ubuntu
        keyPath: /path/.ssh/id_rsa
      uploadBinary: true
      files:
        # This airgap bundle file will be uploaded from the k0sctl
        # host to the specified directory on the target host
        - src: /local/path/to/bundle-file/airgap-bundle-amd64.tar
          dstDir: /var/lib/k0s/images/
          perm: 0755
```

## 3. Ensure pull policy in the k0s.yaml (optional)

Use the following `k0s.yaml` to ensure that containerd does not pull images for k0s components from the Internet at any time.

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  images:
    default_pull_policy: Never
```

## 4. Set up the controller and worker nodes

Refer to the [Manual Install](k0s-multi-node.md) for information on setting up the controller and worker nodes locally. Alternatively, you can use [k0sctl](k0sctl-install.md).

**Note**: During the worker start up k0s imports all bundles from the `$K0S_DATA_DIR/images` before starting `kubelet`.
