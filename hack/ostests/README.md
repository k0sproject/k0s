# Terraform modules for k0s OS testing

Provisioning of k0s test clusters using different operating system stacks and
network providers based on Terraform and k0sctl.

By default, a test cluster will have the following properties:

* 3 controller nodes
* 2 worker nodes
* podCIDR is set to 10.244.0.0/16
* Node-local load balancing is enabled.
* All the rest of the configuration is left at its defaults.

Two workers are a minimum requirement for the Kubernetes conformance tests.

## Requirements

* [Terraform] >= 1.4

For the local plumbing:

* A POSIXish environment (`env`, `sh`, `echo`, `printf`)
* [k0sctl] (tested with 0.17.1)
* [jq] (tested with ~= 1.6)

For the AWS infra:

* Have the CLI credentials setup, in the usual AWS CLI way.
* Have a configured default region. That region will be targeted by Terraform.

[Terraform]: https://developer.hashicorp.com/terraform
[k0sctl]: https://github.com/k0sproject/k0sctl/
[jq]: https://jqlang.github.io/jq/

## Deploying a cluster

Be sure to meet the requisites listed above. Configure AWS credentials and
region. This is just an example, there are [other ways][aws-config] to do it.

```shell
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_SESSION_TOKEN=...
export AWS_REGION=...
```

Select the desired cluster configuration for Terraform. Again, just an example,
other ways described [here][tf-config].

```shell
export TF_VAR_os=alpine_3_17
export TF_VAR_k0s_version=stable
export TF_VAR_k0s_network_provider=calico
export TF_VAR_k0s_kube_proxy_mode=ipvs
```

[aws-config]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[tf-config]: https://developer.hashicorp.com/terraform/language/values/variables#assigning-values-to-root-module-variables

Apply the configuration:

```shell
terraform apply
```

## Terraform configuration variables

### `os`: Operating system stack

* `al2023`: Amazon Linux 2023
* `alpine_3_17`: Alpine Linux 3.17
* `centos_7`: CentOS Linux 7 (Core)
* `centos_8`: CentOS Stream 8
* `centos_9`: CentOS Stream 9
* `debian_10`: Debian GNU/Linux 10 (buster)
* `debian_11`: Debian GNU/Linux 11 (bullseye)
* `debian_12`: Debian GNU/Linux 12 (bookworm)
* `fcos_38`: Fedora CoreOS 38
* `fedora_38`: Fedora Linux 38 (Cloud Edition)
* `flatcar`: Flatcar Container Linux by Kinvolk
* `oracle_7_9`: Oracle Linux Server 7.9
* `oracle_8_7`: Oracle Linux Server 8.7
* `oracle_9_1`: Oracle Linux Server 9.1
* `rhel_7`: Red Hat Enterprise Linux Server 7.9 (Maipo)
* `rhel_8`: Red Hat Enterprise Linux 8.6 (Ootpa)
* `rhel_9`: Red Hat Enterprise Linux 9.3 (Plow)
* `rocky_8`: Rocky Linux 8.7 (Green Obsidian)
* `rocky_9`: Rocky Linux 9.2 (Blue Onyx)
* `ubuntu_2004`: Ubuntu 20.04 LTS
* `ubuntu_2204`: Ubuntu 22.04 LTS
* `ubuntu_2304`: Ubuntu 23.04

### `k0sctl_skip`: Skip k0s provisioning altogether

Just provision the infrastructure, but no k0s cluster. This may be used for
development and testing purposes.

Assuming the AWS credentials are available, it can be used like this:

```sh
terraform init
export TF_VAR_os=alpine_3_17
export TF_VAR_k0sctl_skip=true
terraform apply
terraform output -json | jq -r '
  (.ssh_private_key_filename.value | @sh) as $keyFile
    | .hosts.value[]
    | select(.connection.type = "ssh")
    | . as {name: $host, ipv4: $ip, connection: {username: $user}}
    | "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i \($keyFile) \($user)@\($ip) # \($host)"
'
```

### `k0s_version`: The k0s version to deploy

This may be a fixed version number, "stable" or "latest".

### `k0s_network_provider`: Network provider

* `kuberouter`
* `calico`

### `k0s_kube_proxy_mode`: Mode of operation for kube-proxy

* `iptables`
* `ipvs`

See Kubernetes's [IPVS README] for details.

[IPVS README]: https://github.com/kubernetes/kubernetes/blob/master/pkg/proxy/ipvs/README.md

### Adding a new operating system

* Navigate to [modules/os/](modules/os/) and add a new file `os_<the-os-id>.tf`.
  Have a look at the other `os_*.tf` files for how it should look like.
* Add a new OS entry to [modules/os/main.tf](modules/os/main.tf).
* Update this README.
* Test it: Be sure to have the requisites ready, as described at the top of this
  README, then do `terraform apply -var=os=<the-os-id>`. When done, don't
  forget to clean up: `terraform destroy -var=os=<the-os-id>`.
* Update the [nightly trigger] and [matrix workflow] with the new OS ID.

## GitHub Actions workflows

There's a reusable GitHub Actions workflow available in [ostests-e2e.yaml]. It
will deploy the Terraform resources and perform Kubernetes conformance tests
against the provisioned test cluster.

[ostests-e2e.yaml]: ../../.github/workflows/ostests-e2e.yaml

### Launch a workflow run

There's a [nightly trigger] for the OS testing workflow. It will select and run
a single testing parameter combination each day. There's also a [matrix
workflow] that exposes more knobs and can be triggered manually, e.g. via [gh]:

```console
$ gh workflow run ostests-matrix.yaml --ref some/experimental/branch \
  -f oses='["alpine_3_17"]' \
  -f network-providers='["calico"]'
✓ Created workflow_dispatch event for ostests-matrix.yaml at some/experimental/branch

To see runs for this workflow, try: gh run list --workflow=ostests-matrix.yaml
```

[gh]: https://github.com/cli/cli

## TODO

* Figure out the best/canonical way to change host names of the AWS instances

[nightly trigger]: ../../.github/workflows/ostests-nightly.yaml
[matrix workflow]: ../../.github/workflows/ostests-matrix.yaml
