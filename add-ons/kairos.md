# Kairos Meets k0s: A Meta-Distribution for Kubernetes is Born

Kairos had shipped first-class support for k3s until very long. But if the community wanted something else? They had to rely on community-powered providers. 
There have been providers emerge for kubeadm, nodeadm, microk8s, and even another flavor of k3s. Each one added the ability to run Kubernetes on Kairos, but they were often self-contained efforts. Most didn’t plug into their cloud-init-style configuration, which meant you had to know your way around their specific setup to get them running. Power users and contributors made them work, but new users? Not so much.

Then there’s the [provider-kairos](https://github.com/kairos-io/provider-kairos), which offers a consistent configuration layer across setups and makes Kubernetes orchestration a breeze, especially for decentralized, peer-to-peer deployments powered by EdgeVPN. But it had one limitation: it only worked with k3s.

## Enter k0s: The Zero Friction Kubernetes

For those unfamiliar, k0s is a lightweight, CNCF-certified Kubernetes distribution that bundles everything into a single binary. No OS dependencies. No frills. Just Kubernetes, as it should be — simple to run, easy to maintain, and friendly across bare metal, cloud, edge, and IoT. It supports containerd, Kube-Router by default (with optional Calico), and can run anywhere Linux does. It’s perfect for Kairos.

We sat down together and made it happen. This is the magic of open-source: two CNCF Sandbox Projects, Kairos and k0s, coming together to build something bigger than either could alone.

Now, thanks to this collaboration, you can run k0s using the same Kairos-native cloud-init configuration that previously only supported k3s. That means P2P cluster formation with EdgeVPN, full configuration via YAML, and all the DX goodness you’re used to, with a new Kubernetes engine under the hood.


## What this Means

Kairos just leveled up.

We’re not just a Linux meta-distribution anymore. We’re taking our first real steps into becoming a Kubernetes meta-distribution.

So meta.

From now on, users don’t have to choose between clean, declarative setups and their preferred Kubernetes flavor. The Kairos provider now supports both k3s and k0s natively. Want to try it? We’ve added examples to our repository showing both side-by-side so you can pick the flavor that suits your use case best.

And it doesn’t stop there. As noted in the recent [CNCF blog post](https://www.cncf.io/blog/2025/03/25/building-secure-kubernetes-edge-images-with-kairos-and-k0s/), combining k0s’s single-binary simplicity with Kairos’s secure, immutable images is a huge win for edge computing. Together, we’re making it easier to build secure-by-default, production-ready clusters from the ground up.


## Try it today

The k0s integration will be fully stable in Kairos v3.4, but you don’t have to wait. It’s already available in our beta or nightly images for [Ubuntu](https://quay.io/repository/kairos/ubuntu), [Rockylinux](https://quay.io/repository/kairos/rockylinux), and [openSUSE](https://quay.io/repository/kairos/opensuse), or you can [build your own with AuroraBoot](https://kairos.io/docs/advanced/build/#auroraboot).

To get started:

Check out the [Kairos provider repository](https://github.com/kairos-io/provider-kairos)
Explore the updated examples with k3s and k0s
Spin up a custom image
And let us know what you build!
