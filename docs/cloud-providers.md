# Using cloud providers

k0s builds Kubernetes components in "providerless" mode. This means that there is no cloud providers built into k0s managed Kubernetes components.

This means the cloud providers have to be configured "externally". The following steps outline how to enable cloud providers support in your k0s cluster.

For more information on running Kubernetes with cloud providers see the [official documentation](https://kubernetes.io/docs/tasks/administer-cluster/running-cloud-controller/).

## Enabling cloud provider support in kubelet

Even when all components are built with "providerless" mode, we need to be able to enable cloud provider "mode" for kubelet. This is done by running the workers with `--enable-cloud-provider=true`. This enables `--cloud-provider=external` on kubelet process.

## Deploying the actual cloud provider

Fro Kubernetes point of view, it does not realy matter how and where the cloud providers controller(s) are running. Of course the easiest way is to deploy them on the cluster itself. 

To deploy your cloud provider as k0s managed stack you can use the built-in [manifest deployer](manifests.md). Simply drop all the needed manifests under e.g. `/var/lib/k0s/manifests/aws/` directory and k0s will deploy everything.

Some cloud providers do need some configuration files to be present on all the nodes or some other pre-requisites. Consult your cloud providers documentation for needed steps.

