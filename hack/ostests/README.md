# OpenTofu modules for k0s OS testing

Provisioning of k0s test clusters using different operating system stacks and
network providers based on OpenTofu and k0sctl.

By default, a test cluster will have the following properties:

* 3 controller nodes
* 2 worker nodes
* podCIDR is set to 10.244.0.0/16
* Node-local load balancing is enabled.
* All the rest of the configuration is left at its defaults.

Two workers are a minimum requirement for the Kubernetes conformance tests.

## Requirements

* [OpenTofu] >= 1.8

For the local plumbing:

* A POSIXish environment (`env`, `sh`, `echo`, `printf`)
* [k0sctl] (tested with 0.17.1)
* [jq] (tested with ~= 1.6)

For the AWS infra:

* Have the CLI credentials setup, in the usual AWS CLI way.
* Have a configured default region. That region will be targeted by OpenTofu.

[OpenTofu]: https://opentofu.org
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

Select the desired cluster configuration for OpenTofu. This is just one example;
other methods of passing configuration options are covered in the [official
OpenTofu documentation][tf-config].

```shell
export TF_VAR_os=alpine_3_22
export TF_VAR_arch=x86_64
export TF_VAR_k0s_version=stable
export TF_VAR_k0s_network_provider=calico
export TF_VAR_k0s_kube_proxy_mode=ipvs
```

[aws-config]: https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-configure.html
[tf-config]: https://opentofu.org/docs/language/values/variables/#assigning-values-to-root-module-variables

Apply the configuration:

```shell
tofu apply
```

## OpenTofu configuration variables

### `os`: Operating system stack

* `al2023`: Amazon Linux 2023
* `alpine_3_19`: Alpine Linux 3.19
* `alpine_3_22`: Alpine Linux 3.22
* `centos_9`: CentOS Stream 9
* `centos_10`: CentOS Stream 10 (Coughlan)
* `debian_11`: Debian GNU/Linux 11 (bullseye) ([supported until 2026-08-31][debian-lts])
* `debian_12`: Debian GNU/Linux 12 (bookworm) ([supported until 2028-06-30][debian-lts])
* `fcos_stable`: [Fedora CoreOS stable stream](https://docs.fedoraproject.org/en-US/fedora-coreos/getting-started/#_streams)
* `fedora_41`: Fedora Linux 41 (Cloud Edition)
* `flatcar`: Flatcar Container Linux by Kinvolk
* `oracle_8_9`: Oracle Linux Server 8.9
* `oracle_9_3`: Oracle Linux Server 9.3
* `rhel_7`: Red Hat Enterprise Linux Server 7.9 (Maipo)
* `rhel_8`: Red Hat Enterprise Linux 8.10 (Ootpa)
* `rhel_9`: Red Hat Enterprise Linux 9.7 (Plow)
* `rocky_8`: Rocky Linux 8.10 (Green Obsidian)
* `rocky_9`: Rocky Linux 9.5 (Blue Onyx)
* `sles_15`: SUSE Linux Enterprise Server 15 SP6
* `ubuntu_2004`: Ubuntu 20.04 LTS
* `ubuntu_2204`: Ubuntu 22.04 LTS
* `ubuntu_2404`: Ubuntu 24.04
* `windows_server_2022`: Windows Server 2022 (runs the control plane on Alpine 3.22)

[debian-lts]: https://wiki.debian.org/LTS

### `arch`: Node architecture

The underlying processor architecture for the to-be-provisioned cluster. Note
that not all operating systems support all architectures.

* `x86_64`
* `arm64`

### `k0sctl_skip`: Skip k0s provisioning altogether

Just provision the infrastructure, but no k0s cluster. This may be used for
development and testing purposes.

Assuming the AWS credentials are available, it can be used like this:

```sh
tofu init
export TF_VAR_os=alpine_3_22
export TF_VAR_k0sctl_skip=true
tofu apply
tofu output -json | jq -r '
  (.ssh_private_key_filename.value | @sh) as $keyFile
    | .hosts.value[]
    | select(.connection.type = "ssh")
    | . as {name: $host, ipv4: $ip, connection: {username: $user}}
    | "ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -i \($keyFile) \($user)@\($ip) # \($host)"
'
```

### `k0s_version`: The k0s version to deploy

This may be a fixed version number, "stable" or "latest".

### `k0s_executable_path`: The k0s version to deploy

Path to the k0s executable to use, or null if it should be downloaded. Note that
for Windows, the `.exe` suffix is appended automatically.

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
  README, then do `tofu apply -var=os=<the-os-id>`. When done, don't
  forget to clean up: `tofu destroy -var=os=<the-os-id>`.
* Update the [nightly trigger] and [matrix workflow] with the new OS ID.

### Updating the existing OS versions

New OS version usually mean just updating the OS AMI filtering criteria.
To test the AMI filtering locally, you can do the following:

```sh
tofu apply -refresh-only -target=module.os.data.aws_ami.<the-os-id>
tofu state show "module.os.data.aws_ami.<the-os-id>[0]"
```

## GitHub Actions workflows

There's a reusable GitHub Actions workflow available in [ostests-e2e.yaml]. It
will deploy the OpenTofu resources and perform Kubernetes conformance tests
against the provisioned test cluster.

[ostests-e2e.yaml]: ../../.github/workflows/ostests-e2e.yaml

### Launch a workflow run

There's a [nightly trigger] for the OS testing workflow. It will select and run
a single testing parameter combination each day. There's also a [matrix
workflow] that exposes more knobs and can be triggered manually, e.g. via [gh]:

```console
$ gh workflow run ostests-matrix.yaml --ref some/experimental/branch \
  -f oses='["alpine_3_22"]' \
  -f network-providers='["calico"]'
✓ Created workflow_dispatch event for ostests-matrix.yaml at some/experimental/branch

To see runs for this workflow, try: gh run list --workflow=ostests-matrix.yaml
```

[gh]: https://github.com/cli/cli

## TODO

* Figure out the best/canonical way to change host names of the AWS instances

[nightly trigger]: ../../.github/workflows/ostests-nightly.yaml
[matrix workflow]: ../../.github/workflows/ostests-matrix.yaml
