# Airgap install

In this tutorial we are going to cover k0s deployment in the environment with restricted internet access. 
As usually in the k0sproject we aim to give you the best user experience with the least possible amount of frictions.

## Prerequisites
No specific prerequisites are required.

### Prerequisites for exporting images bundle from running cluster

Working cluster with at least one controller (this cluster will be used to build images bundle).
Please, refer to the [getting started guide](install.md).
You also need to have containerd CLI management tool `ctr` installed on the worker machine. Please refer to the ContainerD [getting-started](https://containerd.io/docs/getting-started/) guide.

## Steps

#### 1. Create OCI bundle
##### 1.1 Using Docker
Use following commands to build OCI bundle by utilizing your docker environment. 
```
## Pull images
# k0s airgap list-images | xargs -I{} docker pull {}

## Create bundle
# docker image save $(k0s airgap list-images | xargs) -o bundle_file
```

##### 1.2 Using previously set up k0s worker
To build OCI bundle with images we can utilize the fact that containerd pulls all images during the k0s worker normal bootstrap.
Use following commands on the machine with previously installed k0s worker:

```
## The command k0s airgap list-images prints images used by the current setup
## It respects the k0s.yaml values

# export IMAGES=`k0s airgap list-images | xargs`
# ctr --namespace k8s.io --address /run/k0s/containerd.sock images export bundle_file $IMAGES 
```

Pay attention to the `address` and `namespace` arguments given to the `ctr` tool.

#### 2. Sync bundle file with airgapped machine

Copy the `bundle_file` from the previous step to the target machine. Place the file under the `images` directory in the k0s data directory.
Use following commands to place bundle into the default location:

```
# mkdir -p /var/lib/k0s/images
# cp bundle_file /var/lib/k0s/images/bundle_file
```

#### 3. Ensure pull policy in the k0s.yaml (optional)

Use the following k0s.yaml to force containerd to never pull images for the k0s components. Otherwise containerd pulls the images, which are not found from the bundle, from the internet.
```
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: k0s
spec:
  images:
    default_pull_policy: Never
```


#### 4. Controller
Set up the controller node as usual. Please refer to the [getting started guide](install.md).

#### 5. Run worker

Do the worker set up as usually on the airgapped machine.
During the start up k0s worker will import all bundles from the `$K0S_DATA_DIR/images` before even starting `kubelet`