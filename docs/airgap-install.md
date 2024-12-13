# Airgapped Installation

You can install k0s in environments without Internet access. Airgapped
installations require an image bundle that contains all the container images
that would normally be pulled over the network. K0s uses so-called OCI archives
for this: Tarball representations of an [OCI Image Layout]. They allow for
multiple images to be packed into a single file. K0s will watch for image
bundles in the `<data-dir>/images` folder will automatically import them into
the container runtime.

There are several ways to obtain an image bundle:

- Use the pre-built image bundles for different target platforms that are
  created for each k0s release. They contain all the images for the default k0s
  [image configuration](configuration.md#specimages) and can be downloaded from
  the [GitHub releases page].
- Create your own image bundle. In this case, you can easily customize the
  bundle to include container images that are not used by default in k0s.

**Note:** When importing image bundles, k0s uses ["loose" platform
matching](https://pkg.go.dev/github.com/containerd/platforms@v0.2.1#Only). For
example, on arm/v8, k0s will also import arm/v7, arm/v6, and arm/v5 images. This
means that your bundle can contain multi-arch images, and the import will be
done using platform compatibility.

[OCI Image Layout]: https://github.com/opencontainers/image-spec/blob/v1.0/image-layout.md
[GitHub releases page]: https://github.com/k0sproject/k0s/releases/v{{{ extra.k8s_version }}}+k0s.0

## Creating image bundles

### Using k0s builtin tooling

k0s ships with the [`k0s airgap`](cli/k0s_airgap.md) sub-command, which is
dedicated for tooling for airgapped environments. It allows for listing the
required images for a given configuration, as well as bundling them into an OCI
Image Layout archive.

1. Create the list of images required by k0s.

   ```shell
   k0s airgap list-images --all >airgap-images.txt
   ```

2. Review this list and edit it according to your needs.

3. Create the image bundle.

   ```shell
   k0s airgap bundle-artifacts -v -o image-bundle.tar <airgap-images.txt
   ```

### From a running worker node

As containerd pulls all the images during the k0s worker normal bootstrap, you
can use it to build the OCI bundle with images.

Use the following commands on a machine with an installed k0s worker:

```shell
k0s ctr images export image-bundle.tar $(k0s airgap list-images | xargs)
```

### Using third-party tools

There are several CLI tools that can help you fetch OCI artifacts and manage OCI
Image Layouts, such as [skopeo], [oras], or [crane]. The following is an example
uses Docker:

[skopeo]: https://github.com/containers/skopeo
[oras]: https://oras.land/
[crane]: https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md

#### Docker

1. Create the list of images required by k0s.

   ```shell
   k0s airgap list-images --all >airgap-images.txt
   ```

2. Review this list and edit it according to your needs.

3. Pull the images.

   ```shell
   xargs -I{} docker pull {} <airgap-images.txt
   ```

4. Create the bundle.

   ```shell
   docker image save -o image-bundle.tar $(xargs <airgap-images.txt)
   ```

## Placing image bundles on worker nodes

### By hand

Copy the `image-bundle.tar` to the target machine into the `images` directory in
the k0s data directory. Copy the bundle only to the worker nodes. Controller
nodes don't use it.

```console
# mkdir -p /var/lib/k0s/images
# cp image-bundle.tar /var/lib/k0s/images/image-bundle.tar
```

### Via k0sctl

As an alternative to the previous step, you can use `k0sctl` to upload image
bundles to worker nodes. `k0sctl` can also be used to upload the k0s binary file
to all nodes. Take a look at this example configuration with one controller and
one worker node to upload k0s binary and an image bundle:

```yaml
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  k0s:
    version: {{{ extra.k8s_version }}}+k0s.0
  hosts:
    - role: controller
      ssh:
        address: <controller-ip-address>
        user: ubuntu
        keyPath: /path/to/.ssh/id_rsa

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
        keyPath: /path/to/.ssh/id_rsa
      uploadBinary: true
      files:
        # This airgap bundle file will be uploaded from the k0sctl
        # host to the specified directory on the target host
        - src: /path/to/airgap-bundle-amd64.tar
          dstDir: /var/lib/k0s/images
          perm: 0755
```

## Disable image pulling (optional)

Use the following k0s configuration to ensure that all pods and pod templates
managed by k0s contain an `imagePullPolicy` of `Never`, ensuring that no images
are pulled from the Internet at any time.

```yaml
apiVersion: k0s.k0sproject.io/v1beta1
kind: ClusterConfig
metadata:
  name: k0s
spec:
  images:
    default_pull_policy: Never
```
