# Conformance Testing

The terraform code included in this directory will help you quickly set-up a k0s kubernetes cluster on AWS to run the kubernetes conformance tests against.

## Requirements

1. [Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli#install-terraform)
2. [K0sctl](https://github.com/k0sproject/k0sctl/#installation) (a k0s configuration tool)

This guide assumes you have the appropriate AWS environment variables exported for authentication (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` and `AWS_SESSION_TOKEN`).

## Running Terraform

Terraform requires the AWS CLI authentication operations. For more details, refer to the [AWS provider documentation](https://registry.terraform.io/providers/hashicorp/aws/latest/docs#environment-variables).

Once the correct variables are exported, the commands below are needed to run terraform:

```shell
export TF_VAR_cluster_name="k0s_conformance" # can be changed to any value
export TF_VAR_k0s_version=latest # refers to release tags from https://github.com/k0sproject/k0s/releases

cd terraform

terraform init
terraform plan # to view the planned changes
terraform apply
```

### Run k0sctl

The terraform command will create a `k0sctl.yaml` file.

Example:

```yaml
apiVersion: "k0sctl.k0sproject.io/v1beta1"
kind: "cluster"
metadata:
  name: "k0s_conformance-b0b14b7c"
spec:
  hosts:
  - k0sBinaryPath: null
    role: "controller"
    ssh:
      address: "63.32.21.232"
      keyPath: "./aws_private.pem"
      user: "ubuntu"
    uploadBinary: true
  - k0sBinaryPath: null
    role: "worker"
    ssh:
      address: "54.216.71.108"
      keyPath: "./aws_private.pem"
      user: "ubuntu"
    uploadBinary: true
  - k0sBinaryPath: null
    role: "worker"
    ssh:
      address: "3.250.52.147"
      keyPath: "./aws_private.pem"
      user: "ubuntu"
    uploadBinary: true
  k0s:
    version: "1.31.2+k0s.0"
```

To deploy a k0s cluster on the AWS machine, run:

```shell
k0sctl apply -c k0sctl.yaml
```

Example output:

```shell
➜ k0sctl apply -c k0sctl.yaml

⠀⣿⣿⡇⠀⠀⢀⣴⣾⣿⠟⠁⢸⣿⣿⣿⣿⣿⣿⣿⡿⠛⠁⠀⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀█████████ █████████ ███
⠀⣿⣿⡇⣠⣶⣿⡿⠋⠀⠀⠀⢸⣿⡇⠀⠀⠀⣠⠀⠀⢀⣠⡆⢸⣿⣿⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀███          ███    ███
⠀⣿⣿⣿⣿⣟⠋⠀⠀⠀⠀⠀⢸⣿⡇⠀⢰⣾⣿⠀⠀⣿⣿⡇⢸⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⠀███          ███    ███
⠀⣿⣿⡏⠻⣿⣷⣤⡀⠀⠀⠀⠸⠛⠁⠀⠸⠋⠁⠀⠀⣿⣿⡇⠈⠉⠉⠉⠉⠉⠉⠉⠉⢹⣿⣿⠀███          ███    ███
⠀⣿⣿⡇⠀⠀⠙⢿⣿⣦⣀⠀⠀⠀⣠⣶⣶⣶⣶⣶⣶⣿⣿⡇⢰⣶⣶⣶⣶⣶⣶⣶⣶⣾⣿⣿⠀█████████    ███    ██████████
k0sctl 0.13.2 Copyright 2021, k0sctl authors.
Anonymized telemetry of usage will be sent to the authors.
By continuing to use k0sctl you agree to these terms:
https://k0sproject.io/licenses/eula
INFO ==> Running phase: Connect to hosts
INFO [ssh] 3.250.52.147:22: connected
INFO [ssh] 54.216.71.108:22: connected
INFO [ssh] 63.32.21.232:22: connected
INFO ==> Running phase: Detect host operating systems
INFO [ssh] 63.32.21.232:22: is running Ubuntu 22.04 LTS
INFO [ssh] 54.216.71.108:22: is running Ubuntu 22.04 LTS
INFO [ssh] 3.250.52.147:22: is running Ubuntu 22.04 LTS
INFO ==> Running phase: Acquire exclusive host lock
INFO ==> Running phase: Prepare hosts
INFO ==> Running phase: Gather host facts
.
.
.
INFO [ssh] 3.250.52.147:22: uploading k0s binary from /home/ubuntu/.cache/k0sctl/k0s/linux/amd64/k0s-v1.31.2+k0s.0
INFO [ssh] 63.32.21.232:22: uploading k0s binary from /home/ubuntu/.cache/k0sctl/k0s/linux/amd64/k0s-v1.31.2+k0s.0
INFO [ssh] 54.216.71.108:22: uploading k0s binary from /home/ubuntu/.cache/k0sctl/k0s/linux/amd64/k0s-v1.31.2+k0s.0
INFO ==> Running phase: Configure k0s
WARN [ssh] 63.32.21.232:22: generating default configuration
INFO [ssh] 63.32.21.232:22: validating configuration
INFO [ssh] 63.32.21.232:22: configuration was changed
INFO ==> Running phase: Initialize the k0s cluster
INFO [ssh] 63.32.21.232:22: installing k0s controller
INFO [ssh] 63.32.21.232:22: waiting for the k0s service to start
INFO [ssh] 63.32.21.232:22: waiting for kubernetes api to respond
INFO ==> Running phase: Install workers
INFO [ssh] 54.216.71.108:22: validating api connection to https://10.0.57.78:6443
INFO [ssh] 3.250.52.147:22: validating api connection to https://10.0.57.78:6443
INFO [ssh] 63.32.21.232:22: generating token
INFO [ssh] 54.216.71.108:22: writing join token
INFO [ssh] 3.250.52.147:22: writing join token
INFO [ssh] 3.250.52.147:22: installing k0s worker
INFO [ssh] 54.216.71.108:22: installing k0s worker
INFO [ssh] 3.250.52.147:22: starting service
INFO [ssh] 54.216.71.108:22: starting service
INFO [ssh] 54.216.71.108:22: waiting for node to become ready
INFO [ssh] 3.250.52.147:22: waiting for node to become ready
INFO ==> Running phase: Release exclusive host lock
INFO ==> Running phase: Disconnect from hosts
INFO ==> Finished in 1m42s
INFO k0s cluster version v1.31.2+k0s.0 is now installed
INFO Tip: To access the cluster you can now fetch the admin kubeconfig using:
INFO      k0sctl kubeconfig
```

### Run Conformance

#### Get Kubeconfig Testing

```shell
k0sctl kubeconfig > k0s_kubeconfig
export KUBECONFIG=./k0s_kubeconfig
```

### Run Sonobuoy

```shell
cd inttest # make sure you are in the inttest directory
make check-conformance
```

Example Output:

```shell
➜ make check-conformance
/home/ubuntu/k0s/inttest/bin/sonobuoy run --wait=1200 \
        --mode=certified-conformance \
        --plugin-env=e2e.E2E_EXTRA_ARGS="--ginkgo.v" \
        --kubernetes-version=v1.31.2
INFO[0000] create request issued                         name=sonobuoy namespace= resource=namespaces
INFO[0000] create request issued                         name=sonobuoy-serviceaccount namespace=sonobuoy resource=serviceaccounts
INFO[0000] create request issued                         name=sonobuoy-serviceaccount-sonobuoy namespace= resource=clusterrolebindings
INFO[0000] create request issued                         name=sonobuoy-serviceaccount-sonobuoy namespace= resource=clusterroles
INFO[0000] create request issued                         name=sonobuoy-config-cm namespace=sonobuoy resource=configmaps
INFO[0000] create request issued                         name=sonobuoy-plugins-cm namespace=sonobuoy resource=configmaps
INFO[0000] create request issued                         name=sonobuoy namespace=sonobuoy resource=pods
INFO[0000] create request issued                         name=sonobuoy-aggregator namespace=sonobuoy resource=services
11:41:13          PLUGIN       NODE    STATUS   RESULT   PROGRESS
11:41:13             e2e     global   running
11:41:13    systemd-logs   worker-0   running
11:41:13    systemd-logs   worker-1   running
11:41:13
11:41:13 Sonobuoy is still running. Runs can take 60 minutes or more depending on cluster and plugin configuration.
11:41:33             e2e     global    running            Passed:  1, Failed:  0, Remaining:355
11:41:33    systemd-logs   worker-0   complete
11:41:33    systemd-logs   worker-1   complete
11:41:53             e2e     global    running            Passed:  3, Failed:  0, Remaining:353
...
...
11:42:53             e2e     global    running            Passed:  5, Failed:  0, Remaining:351
...
11:43:33             e2e     global    running            Passed:  7, Failed:  0, Remaining:349
...
11:44:13             e2e     global    running            Passed:  8, Failed:  0, Remaining:348
11:44:33             e2e     global    running            Passed:  9, Failed:  0, Remaining:347
```

#### Fetching Sonobuoy Results

```shell
make get-conformance-results

# command output
# /home/ubuntu/k0s/inttest/bin/sonobuoy retrieve
# 202208011140_sonobuoy_71cd95a4-2d01-421c-bb0f-b4a380dfb6d4.tar.gz
```

```shell
bin/sonobuoy results 202208011140_sonobuoy_71cd95a4-2d01-421c-bb0f-b4a380dfb6d4.tar.gz

# command output:
# Plugin: e2e
# Status: passed
# Total: 6971
# Passed: 356
# Failed: 0
# Skipped: 6615

# Plugin: systemd-logs
# Status: passed
# Total: 2
# Passed: 2
# Failed: 0
# Skipped: 0
```
