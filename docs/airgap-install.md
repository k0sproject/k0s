# Airgap install

You can install k0s in an environment with restricted Internet access.

## Prerequisites

* A working cluster with at least one controller, to be used to
  build the images bundle. For more information, refer to the [Quick Start Guide](install.md).

* The containerd CLI management tool `ctr`, installed on the worker machine (refer to the ContainerD [getting-started](https://containerd.io/docs/getting-started/) guide). 

### 1. Create the OCI bundle

**Note**: k0s supports only uncompressed image bundles.

As OCI bundles are build specifically for each architecture, create an OCI bundle that uses the same processor architecture (x86-64, ARM64, ARMv7) as on the target system.

k0s offers two methods for creating OCI bundles, one using Docker and the other using a previously set up k0s worker. Be aware, though, that you cannot use the Docker method for the ARM architectures due to [kube-proxy image multiarch manifest problem](https://github.com/kubernetes/kubernetes/issues/98229). 

#### Docker

1. Pull the images.

   ```
   k0s airgap list-images | xargs -I{} docker pull {}
   ```

2. Create the bundle.

   ```
   docker image save $(k0s airgap list-images | xargs) -o bundle_file
   ```

#### Previously set up k0s worker

As containerd pulls all images during the k0s worker normal bootstrap, you can use it to build the OCI bundle with images. 

Use following commands on a machine with an installed k0s worker:

```
# export IMAGES=`k0s airgap list-images | xargs`
# ctr --namespace k8s.io --address /run/k0s/containerd.sock images export bundle_file $IMAGES
```

### 2. Sync the bundle file with the airgapped machine

Copy the `bundle_file` you created in the previous step to the target machine into the `images` directory in the k0s data directory. 

```
# mkdir -p /var/lib/k0s/images
# cp bundle_file /var/lib/k0s/images/bundle_file
```

### 3. Ensure pull policy in the k0s.yaml (Optional)

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

### 4. Set up the Controller

Refer to the [Quick Start Guide](install.md) for information on setting up the controller node. 

### 5. Set up a worker

Perform the worker set up on the airgapped machine.

**Note**: During start up the k0s worker imports all bundles from the `$K0S_DATA_DIR/images` before even starting `kubelet`.