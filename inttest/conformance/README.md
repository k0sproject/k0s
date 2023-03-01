# Conformance Testing

This is a TF configuration to easily setup a K0s cluster with AWS and start sonobuoy.
Requirements:

1. Terraform
2. AWS credentials

To provision required setup follow next steps:

```shell
$ cd terraform
$ terraform init
$ terraform apply
module.k0s-sonobuoy.data.aws_ami.ubuntu: Refreshing state...
module.k0s-sonobuoy.tls_private_key.k8s-conformance-key: Creating...
module.k0s-sonobuoy.tls_private_key.k8s-conformance-key: Creation complete after 1s [id=07fae4a3e454177b6156c3342d6d92008426d703]
module.k0s-sonobuoy.local_file.aws_private_pem: Creating...
...
Apply complete! Resources: 17 added, 0 changed, 0 destroyed.

Outputs:

controller_ip = [
  "54.73.141.241",
]
```

## Test Variables

In order to run the conformance test, you will need to set the tested k0s version and the tested Kubernetes version. This can be done in two ways.

### 1. Create a var file

In the same directory as your `main.tf` file, create an additional file `terraform.tfvars` with the following input:

```terraform
k0s_version=v1.26.2+k0s.0
k8s_version=v1.26.2
sonobuoy_version=0.53.2
```

### 2. Environment variables

```shell
TF_VAR_k0s_version=v1.26.2+k0s.0 TF_VAR_sonobuoy_version=0.20.0 TF_VAR_k8s_version=v1.26.2 terraform apply
```

**NOTE:** By default, terraform will fetch sonobuoy version **0.53.2**. If you want to use a different version you can override this with one of the above methods.

## Fetching Sonobuoy's results

Once provisioning of the cluster finishes you can get the results by SSH'ing into the controller:

```shell
$ ssh -i .terraform/modules/k0s-sonobuoy/inttest/terraform/test-cluster/aws_private.pem ubuntu@[controller_ip]

ubuntu@controller-0:~$ export KUBECONFIG=/var/lib/k0s/pki/admin.conf
ubuntu@controller-0:~$ sudo --preserve-env=KUBECONFIG sonobuoy status
         PLUGIN     STATUS   RESULT   COUNT               PROGRESS
            e2e    running                1   128/346 (0 failures)
   systemd-logs   complete                3

Sonobuoy is still running. Runs can take 60 minutes or more depending on cluster and plugin configuration.
```

Once sonobuoy finishes to retieve results run following command on the cluster host:

```shell
result=$(sudo --preserve-env=KUBECONFIG sonobuoy retrieve)
```

Analyze results:

```shell
$ sudo --preserve-env=KUBECONFIG sonobuoy results $results
Plugin: systemd-logs
Status: passed
Total: 3
Passed: 3
Failed: 0
Skipped: 0

Plugin: e2e
Status: failed
Total: 5233
Passed: 302
Failed: 1
Skipped: 4930
```

To retrieve results to you localhost run following command:

```shell
scp -i .terraform/modules/k0s-sonobuoy/inttest/terraform/test-cluster/aws_private.pem ubuntu@[controller_ip]:[path_to_results_tarball]
```

And finally to teardown the cluster run:

```shell
terraform destroy --auto-approve
```
