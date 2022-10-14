# Tool

This `tool` marries the functionalities of `terraform` with `k0sctl`, giving you a single command for creating a `k0s`
cluster along with the infrastructure hosting it (currently AWS EC2)

## Examples

For this tool, examples are the best documentation.

### Creating an HA development `k0s` cluster on AWS (1x3)

This uses the `ha` command of the `aws` provider.

Anyone who wants a quick HA cluster to do experiments can use this, assuming that they have AWS credentials

Prerequisite: Ensure that your environment has `AWS_ACCESS_KEY_ID`, `AWS_SESSION_TOKEN`, and `AWS_SECRET_ACCESS_KEY`
exported in your shell environment.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/mycluster/hadev1x3

./tool.sh aws ha create \
    --cluster-name hadev1x3 \
    --region ca-central-1 \
    --k0s-version v1.25.2+k0s.0 \
    --controllers 1 \
    --workers 3
```

### Creating an HA development `k0s` cluster on AWS (1x3), using my own `k0s` binary

To test your local `k0s` changes against a real-world cluster, if you copy the binary into your `TOOL_DATADIR`,
and use the `--k0s-binary` flag, the underlying `k0sctl` will upload the binary during the installation of `k0s`
on the cluster.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/mycluster/hadev1x3testing

# The dev binary needs to live in ${TOOL_DATADIR}
cp /path/to/my/k0s ${TOOL_DATADIR}

# The `--k0s-binary` value should be the same filename (without path) as your binary
./tool.sh aws ha create \
    --cluster-name hadev1x3 \
    --region ca-central-1 \
    --k0s-binary k0s \
    --controllers 1 \
    --workers 3
```

### Creating an HA development `k0s` cluster on AWS (1x3) using an Airgap image bundle

Similar to the non-airgap bundle instructions, this adds arguments for specifying an airgap bundle
along with a YAML image manifest.

Prerequisites:

* Ensure that the files used for the `k0s` binary and airgap bundle exist in your `TOOL_DATADIR`.
* A YAML manifest of the images is required.
  * This can be created using a `spec.images` object
    * https://docs.k0sproject.io/head/configuration/#specimages
  * A minimal YAML can consist of simply `default_pull_policy: Never`
* NOTE: The values for `--k0s-airgap-bundle` and `--k0s-airgap-bundle-config` reference the names of files in `TOOL_DATADIR`,
and not full paths.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/mycluster/hadev1x3

./tool.sh aws ha create \
    --cluster-name hadev1x3 \
    --region ca-central-1 \
    --k0s-version v1.25.2+k0s.0 \
    --controllers 1 \
    --workers 3 \
    --k0s-airgap-bundle k0s-airgap-bundle-v1.25.2+k0s.0-amd64 \
    --k0s-airgap-bundle-config images.yaml
```

### Creating an array of HA `k0s` clusters in an isolated VPC

This uses the `havpc` command of the `aws` provider.

The use case for this example is a set of integration tests that `k0s` would want to run against a collection
of isolated and independent `k0s` clusters.

#### Create the VPC

To have complete control over the integration test network, we create our own AWS VPC which will will then split
into assorted subnets for each cluster.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/inttests/infra

./tool.sh aws vpcinfra create \
    --name inttests-infra \
    --region ca-central-1 \
    --cidr 10.1.0.0/16
```

NOTE: The output of this command is the `vpc_id` which is needed for cluster creation.

#### Create a `k0s` cluster at subnet 0

This will create a subnet of `10.1.0.0/26`, and `k0s` will be installed there.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/inttests/inttest0

./tool.sh aws havpc create \
    --cluster-name inttest0 \
    --region ca-central-1 \
    --vpc-id <vpc_id from step above> \
    --subnet-idx 0 \
    --k0s-version v1.25.2+k0s.0 \
    --controllers 3 \
    --workers 3
```

#### Creating additional `k0s` cluster at subnet N

This is basically the same command as above, however with `subnet-idx` changed to `N`, and the
new `cluster-name` and `TOOL_DATADIR` names.

```bash
# This directory is where the terraform state and private key are saved.
export TOOL_DATADIR=/inttests/inttestN

./tool.sh aws havpc create \
    --cluster-name inttestN \
    --region ca-central-1 \
    --vpc-id <vpc_id from step above> \
    --subnet-idx N \
    --k0s-version v1.25.2+k0s.0 \
    --controllers 3 \
    --workers 3
```

## Interface

A hard interface is intentionally not defined here as different cloud/infrastructure providers may require additional
commands or files.

At a minimum:

* The `TOOL_DATADIR` is a 1:1 association to the command you plan to use.
  * Any cluster accessing material (kubeconfig, private keys, etc) can be saved here.
  * Volume-mapped to your host

## Caveats

### Terraform

The functionality provided relies heavily on `terraform` and the concepts of `terraform`, so there are a number of
things to be immediately aware of:

* If you manually change infrastructure after it has been made with `create`, you may have a hard time using `destroy`.
* `terraform` saves state (your `TOOL_DATADIR` directory)
* The same arguments *should* be used between both `create` and `destroy`
  * Failing to do so could result in remote resources not getting destroyed.
* If a `create` or `destroy` fails, retrying the operation *should* continue where it failed.
* You need to destroy all of your `havpc` resources first before attempting to destroy `vpcinfra`

### Network Sizes

* Its assumed that the VPC network size will be `/16`, and the subnetting will bump that to `/26`
