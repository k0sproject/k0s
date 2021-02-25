# Air-gap installation

Air-gap or offline setup is now possible with k0s.

To engage it we use OCI bundle for containerd to provide images to the cluster before even starting kubelet.

There are few ways to achieve offline setup:

- Use OCI bundle with default images
- Use custom OCI bundle and [override image names](https://docs.k0sproject.io/latest/configuration/#images)
  through `k0s.yaml`
- Setup private registry and override image names (not covered by this document)

## Bundle preparation

You need following images in the bundle:
```
us.gcr.io/k8s-artifacts-prod/kas-network-proxy/proxy-agent:v0.0.13
gcr.io/k8s-staging-metrics-server/metrics-server:v0.3.7
k8s.gcr.io/kube-proxy:v1.20.4
docker.io/coredns/coredns:1.7.0
docker.io/calico/cni:v3.16.2
docker.io/calico/pod2daemon-flexvol:v3.16.2
docker.io/calico/node:v3.16.2
docker.io/calico/kube-controllers:v3.16.2
k8s.gcr.io/pause:3.2
``` 

To generate images list you can use k0s command:
```k0s airgap list-images```

You need to install containerd cli tool [ctr](https://containerd.io/downloads/)

### Default bundle
Manually setup k0s cluster with at least one worker and use the following command to export images from the cluster:

```
export IMAGES=$(k0s airgap list-images | xargs)
ctr --namespace k8s.io --address /run/k0s/containerd.sock images export bundle.tgz $IMAGES
```

### Custom bundle

Custom bundle could be useful in case if you need to pre-install any helm based extensions or apply custom kubernetes manifests.
Just extend the images list received from k0s command with any custom images.

## Starting up worker with a bundle
```k0s worker --airgap-bundle=<path/to> <token>```
## Fully offline mode

To ensure that kubelet uses only images from the bundle use corresponding k0s.yaml:

```
apiVersion: k0s.k0sproject.io/v1beta1
kind: Cluster
images:
  default_pull_policy: Never
```

Important notification here is that `default_pull_policy` affects only images installed by the k0s itself, not images from the helm extenstions or any other manually installed images.

## Private registry

Private registry is not covered by this documentation.