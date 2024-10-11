# Adopters of k0s

This page lists, in alphabetical order, individuals and organizations that are
using k0s in production. If you would like to add your use case for k0s to the
list, we'd be more than happy to see a pull request.

## Adopters

<!--
When adding new adopters, please adhere roughly to the following format:

* <Organization name including link><br>
  Project: <The project name or a short description of the use case><br>
  Contact: <Contact info, if applicable, e.g. email addresses, social media handles ...><br>
  <A longer descriptive text, preferably hard wrapped at around 80-120 characters per line, or at the end of a sentence.>

-->

* [AudioCodes](https://audiocodes.com)<br>
  Project: Underlying support for AudioCodes VoI/VoiceAI<br>
  AudioCodes uses it for [VoIP/VoiceAI (see page 17)](https://www.audiocodes.com/media/mpghsv0o/agent-assist-bot-installation-guide.pdf).

* [DeepSquare](https://deepsquare.io)<br>
  Project: HPCaaS<br>
  DeepSquare embeds it into their HPCaaS [service](https://deepsquare.io/wp-content/uploads/2023/05/DeepSquare_White-Paper-1.pdf).

* [k0smotron](https://k0smotron.io/)<br>
  Project: Managing hosted k0s clusters and full Cluster API provider for k0s<br>
  K0smotron focuses on managing hosted k0s clusters within an existing
  Kubernetes cluster. It acts as a Cluster API provider, enabling seamless
  provisioning, scaling, and lifecycle management of k0s control planes. By
  leveraging the native capabilities of Kubernetes, k0smotron simplifies
  multi-cluster operations and provides flexibility in connecting worker nodes
  from different infrastructures.

* [@k0sproject](https://github.com/k0sproject)<br>
  Project: k0s build and CI infrastructure<br>
  k0s maintainers use k0s to host build and CI infrastructure, mainly dynamic
  Github Runners.

* [KubeArmor](https://docs.kubearmor.io/kubearmor/quick-links/support_matrix)<br>
  Project: Supported in their security product

* [Mirantis](https://www.mirantis.com/software/k0s/)<br>
  Project: k0s support<br>
  Mirantis provides support for various customers utilizing k0s in their
  production environments and k0s is included in a number of Mirantis products
  such as MKE.

* [National Astronomical Observatory for Japan](https://subarutelescope.org)<br>
  Project: Providing compute nodes in telemetry HPC cluster<br>
  Used for deploying and managing
  [NVIDIA GPUs for data analysis](https://subarutelescope.org/Science/SubaruUM/SubaruUM2022/_src/679/P08_Morishima.pdf).

* [Progress Chef 360](https://docs.chef.io/360/1.0/)<br>
  Project: Embedded Clusters for Chef 360<br>
  [Using it for embedded Kubernetes clusters](https://docs.chef.io/360/1.0/install/server/requirements/#kubernetes-requirements).

* [Replicated, Inc.](https://www.replicated.com/)<br>
  Project: Embedded Cluster<br>
  Contact: [Chris Sanders](https://github.com/chris-sanders)<br>
  Replicated builds their [Embedded Cluster](https://docs.replicated.com/vendor/embedded-overview) project on top
  of k0s. Replicated Embedded Cluster allows you to distribute a Kubernetes
  cluster and your application together as a single appliance, making it easy
  for enterprise users to install, update, and manage the application and the
  cluster in tandem.

* [Splunk](https://splunk.com)<br>
  Project: Data Stream Processor<br>
  Used in their [Data Stream Processor](https://docs.splunk.com/Documentation/DSP/1.4.5/Admin/Install).

* [vCluster](https://www.vcluster.com/)<br>
  Project: Virtual Clusters<br>
  vCluster supports both k0s and k3s for their virtual kubernetes nested clusters.

## Additional use cases

In addition, it is being used for novel use cases in the wild:

* [Kubernetes vs Philippine Power Outages - On setting up k0s over Tailscale](https://justrox.me/kubernetes-vs-philippine-power-outages-a-simple-guide-to-k0s-over-tailscale/)
* New England Research Cloud [provides it as an option vs k3s](https://nerc-project.github.io/nerc-docs/other-tools/kubernetes/k0s/).
* Amaze Systems [job posting includes requirement for k0s experience](https://www.salary.com/job/amaze-systems-inc/hiring-for-ml-engineer-data-scientist-robotics-software-engineer-boston-ma-onsite-from-day-1/j202305270140264589562).
* k0s with Traefik for a Tokyo [smart cities project](https://community.traefik.io/t/help-setting-up-with-k0s-via-helm-extensions/20748).
