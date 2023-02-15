# Releases

## Release and support model

Starting from the k0s 1.21, k0s started following the Kubernetes project's [release and support model](https://kubernetes.io/releases/). This means that k0s project will support the last three Kubernetes minor releases. Each minor release will receive approximately 1 year of patch support. In addition to Kubernetes, the k0s patch releases include updates from containerd, runc, etcd, kine, konnectivity, Kube-Router, Calico, CoreDNS, Metrics server etc, and k0s itself, to keep the whole stack updated.

## New features and enhancements

The biggest new k0s features will typically only be delivered on top of the latest Kubernetes version, but smaller enhancements can be included in older release tracks as well.

## Version string

The k0s version string consists of the Kubernetes version and the k0s version. For example:

- v{{{ extra.k8s_version }}}+k0s.0

The Kubernetes version ({{{ extra.k8s_version }}}) is the first part, and the last part (k0s.0) reflects the k0s version, which is built on top of the certain Kubernetes version.
