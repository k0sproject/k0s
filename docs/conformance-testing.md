# Kubernetes conformance testing for k0s

We run the conformance testing for the last RC build for a release. Follow the [instructions](https://github.com/cncf/k8s-conformance/blob/master/instructions.md) as the conformance testing repository.

In a nutshell, you need to:

- Setup k0s on some VMs/bare metal boxes
- Download, if you do not already have, [sonobuoy](https://github.com/vmware-tanzu/sonobuoy) tool
- Run the conformance tests with something like `sonobuoy run --mode=certified-conformance`
- Wait for couple hours
- Collect results
