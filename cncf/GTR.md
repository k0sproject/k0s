# General Technical Review (GTR)

This document collects information for the [CNCF General Technical Review]. It is built to support the [k0s CNCF Sandbox application] but will be a living document that we update regularly.

[CNCF General Technical Review]: https://github.com/cncf/toc/blob/main/tags/resources/toc-supporting-guides/general-technical-questions.md
[k0s CNCF Sandbox application]: https://github.com/cncf/sandbox/issues/125

As we are currently applying for the sandbox level, the document only covers the GTR's “Day 0” phase.

## Scope

### Roadmap process

The k0s roadmap process is a collaborative effort to align with user needs, community feedback, and emerging trends. We prioritize initiatives that simplify Kubernetes operations and extend its capabilities for far-edge, IoT, and disconnected environments.
The roadmap is a living document, refined iteratively based on technical feasibility, testing, and industry shifts. By emphasizing usability, automation, and feedback-driven iterations, we ensure that k0s continues to lead in lightweight, scalable Kubernetes solutions.

### Target personas

#### DevOps Engineers

* Focused on managing Kubernetes clusters efficiently.
* Seek simplicity in deployment, maintenance, and scaling.
* Prioritize automation and minimize operational overhead.

#### Platform Engineers

* Responsible for building and maintaining internal platforms on top of Kubernetes.
* Interested in integrating k0s into CI/CD pipelines, monitoring systems, and other tooling.

#### IoT and Edge Architects

* Working on deploying Kubernetes at the far edge.
* Require lightweight, resource-efficient solutions for disconnected or resource-constrained environments.

#### Cloud-Native Developers

* Building and deploying applications on Kubernetes.
* Need a developer-friendly experience with minimal setup and maximum portability.

#### System Integrators

* Designing bespoke solutions that leverage Kubernetes in varied environments, including on-premises and hybrid setups.
* Value flexibility and compatibility with existing tools and standards.

#### IT Operations Teams

* Managing infrastructure at scale across data centers, cloud, and edge locations.
* Seek reliability, scalability, and tools to simplify multi-cluster operations.

### Primary use cases

**k0s** is a streamlined, self-contained Kubernetes distribution designed to serve as a portable and lightweight foundational core for Kubernetes environments. It simplifies deployment and management, while maintaining the flexibility to build out additional functionality.
As such, it's tailor-made for a variety of use cases, including:

#### Edge Computing and IoT

Deploying Kubernetes in resource-constrained, far-edge environments where lightweight and efficient solutions are critical.
Ideal for managing IoT devices, disconnected edge nodes, and remote locations.
k0s also support airgapped environments by beaing able to automatically load container image bundles and having zero dependencies.

#### Cloud-Native Application Hosting

Enabling developers to run containerized applications with minimal operational complexity.
Suitable for both small-scale setups and large-scale production workloads.

#### Dev/Test Environment

Providing a quick, easy-to-set-up Kubernetes environment for development, testing, and CI/CD pipelines.
Allows developers to spin up Kubernetes clusters locally or in isolated environments without significant overhead.

#### Centralized Control for Distributed Clusters

Offering the ability to centrally manage control planes while operating clusters in remote or disconnected locations.
Ensures operational simplicity across distributed systems.

#### Heterogeneous environments

Running Kubernetes efficiently on bare-metal servers or in hybrid cloud/on-premises environments.
Tailored for teams looking to minimize dependency on external cloud services.
Focus on lightweight deployment without the need for heavy dependencies on external cloud services.

#### Isolated control planes

k0s uses [konnectivity] by default and is therefore a good fit for environments where control planes and worker nodes are isolated.
This architecture is good fit for e.g. Edge type use cases or where users want to cenrtalize their control planes in general.

[konnectivity]: https://github.com/kubernetes-sigs/apiserver-network-proxy

## Unsupported use cases

While k0s provides a self-contained core for Kubernetes operations, it does not aim to be a feature-complete "all-in-one" solution. In particular, the following are out of scope:

### Bundling of higher-level ecosystem components

k0s does not include additional tools such as ingress controllers, service meshes, or advanced observability capabilities by default. Cluster administrators are expected to install and configure these components separately using the extension points provided by k0s, according to their unique requirements.

### Being a "Turnkey Solution"

k0s avoids bundling excessive features to remain lightweight and customizable. It is best suited for those who value flexibility over out-of-the-box completeness.

## Intended types of organizations

The intended types of organizations for using **k0s** are those that prioritize simplicity, flexibility, and efficiency in Kubernetes deployments. Some examples:

### Small and Medium-Sized Businesses (SMBs)

* Looking for a straightforward, cost-effective Kubernetes solution.
* Often lack dedicated Kubernetes experts and benefit from k0s’s ease of deployment and management.

### Edge Computing and IoT Organizations

* Deploying Kubernetes at the far edge to manage IoT devices, industrial equipment, or remote sites.
* Require lightweight, resource-efficient solutions that can operate in disconnected or low-bandwidth environments.

### Software Development Teams and Startups

* Need fast, simple Kubernetes clusters for development, testing, and CI/CD pipelines.
* Value the minimal overhead of k0s for prototyping and scaling quickly.

### Enterprises with Distributed Operations

* Manage hybrid setups with centralized control planes and distributed worker nodes across on-premises, cloud, and edge environments.
* Use k0s to simplify cluster management across multiple locations.

### Organizations in Regulated Industries

* Operate Kubernetes in on-premises or air-gapped environments due to regulatory requirements.
* 's lean design of reducing unnecessary complexity, making it easier to audit and verify compliance with regulatory standards

### Educational and Research Institutions

* Need Kubernetes clusters for experimentation, training, or academic research.
* Value k0s’s simplicity for non-commercial use cases and resource-constrained setups.

### Software Vendors and System Integrators

* Deliver Kubernetes-based solutions to clients across various industries.
* Bundle and package k0s with their applications to create "Kubernetes appliances."
* Ship pre-configured, self-contained solutions for customers without requiring Kubernetes expertise.
* Appreciate k0s’s ease of deployment, lightweight architecture, and flexibility to adapt to different environments.
* Benefit from its embedded-friendly design, making it ideal for turnkey application deployments in diverse scenarios.

## Completed end-user research

No comprehensive end-user research has been conducted.

## Usability

k0s simplifies Kubernetes operations by delivering the entire distribution as a single, self-contained binary. This design ensures:

1. **Streamlined Installation**
   * Deploy Kubernetes with a single command, without the need to manage multiple dependencies or configurations.
2. **Portability and Flexibility**
   * The binary can be easily copied and run on various systems, making it ideal for edge devices, bare-metal servers, or cloud environments.
3. **Simplified Upgrades**
   * Upgrading k0s is as easy as replacing the binary, reducing downtime and operational complexity. To help in orchestrating the cluster upgrade k0s comes with a component called [autopilot] that automates the node ugrade orchestration, including the needed node draining and coordination.
4. **Reduced Operational Overhead**
   * No need for additional packaging or complex tooling — everything needed to run Kubernetes is included in one binary.
5. **Developer and Operator Friendly**
   * The single-binary approach removes barriers to entry, enabling faster adoption and simpler workflows for teams of any size.

As such, k0s provides a conformant Kubernetes which means any ecosystem addon works on k0s.

[autopilot]: https://docs.k0sproject.io/stable/autopilot/

## Design

### Design principles & best practices

#### Design Principles

1. **Simplicity First**
   * k0s is designed to minimize complexity for both users and operators. Packaging Kubernetes as a single binary and automating common tasks eliminates unnecessary configuration overhead.
2. **Lightweight and Efficient**
   * Optimized for resource-constrained environments, k0s ensures minimal system resource usage, making it ideal for edge computing, IoT, and small-scale deployments.
3. **Decoupled and Modular Architecture**
   * k0s separates the control plane and worker components, allowing flexible deployment topologies. It supports running control planes centrally while distributing worker nodes across diverse locations.
4. **Zero Friction, Zero Lock-In**
   * k0s follows a vendor-neutral approach, ensuring users retain full control of their infrastructure. It avoids proprietary tooling, adhering to upstream Kubernetes standards.
5. **Security by Default**
   * Secure configurations are baked into k0s from the start, with features like automatic TLS management, disabling anonymous access etc..
6. **Ease of Maintenance and Upgrades**
   * The project prioritizes operational simplicity, with upgrades streamlined through single-binary replacements, embedded autopilot for upgrades, and minimal manual intervention.
7. **Adaptability**
   * Designed to run in diverse environments, from local development setups to far-edge and production-grade clusters.

### **Best Practices**

1. **Stay Aligned with Upstream Kubernetes**
   * k0s is and will be 100% vanilla upstream Kubernetes.
2. **Automate Wherever Possible**
   * Automation of cluster configuration, control plane management, and other operational tasks ensures reliability and reduces human error.
3. **Test for Real-World Use Cases**
   * Every feature and release is tested in scenarios reflecting actual user environments, such as edge deployments, air-gapped setups, and hybrid clusters.
4. **Community-Driven Development**
   * Open communication with the user community drives prioritization and improvements, ensuring the project meets real-world needs.
5. **Focus on Documentation**
   * Clear and comprehensive documentation ensures users of all skill levels can deploy, manage, and scale k0s effectively.

## Identity and Access Management

k0s being vanilla upstream Kubernetes thus it supports all the same things as [Kubernetes does](https://kubernetes.io/docs/concepts/security/).
By default, k0s only sets up certificate authentication but users can fully configure OIDC or webhook authentiction when they need.

## Compliance requirements implemented

## HA Requirements

1. **Control Plane Redundancy**
   * To achieve HA, the control plane (API server, etcd, controller manager, and scheduler) must be deployed in a redundant configuration:
     * **Multiple Control Plane Nodes**: At least three control plane nodes are recommended to ensure quorum and fault tolerance for etcd. In case kine is used with HA SQL database backend, 2 controller nodes are often sufficient.
     * **Load Balancer**: A load balancer is required in front of the control plane nodes to distribute API traffic evenly and provide failover.
       * k0s offers an embedded control plane load balancer ([CPLB]) for use cases where the infrastructure/network does not provide easy ways to create load balancers on its own.
       * K0s has a node-local ([NLLB]) for worker nodes to automatically re-route API connections to different nodes in case of failures.
2. **Worker Node Scalability**
   * Worker nodes operate independently of the control plane. In an HA setup:
     * Worker nodes can connect to multiple control plane endpoints for resilience.
     * Horizontal scaling of worker nodes ensures workload availability and supports increased demand.
3. **Distributed etcd**
   * etcd, the data store for Kubernetes, requires three or five instances for HA to maintain quorum.
   * etcd instances are distributed across control plane nodes, ensuring data replication and consistency even if a node fails.
   * k0s supports externally (from k0s point of view) managed etcd.
4. **Kine**
   * k0s also supports [kine] as etcd replacement thus offering users the ability to utilize externally managed HA databases as the control plane state storage.

[CPLB]: https://docs.k0sproject.io/stable/cplb/
[NLLB]: https://docs.k0sproject.io/stable/nllb/
[kine]: https://github.com/k3s-io/kine/

## Resource requirements

**k0s** is designed to be lightweight, making it suitable resource-constrained systems. Below is an overview of the resource requirements, including CPU, memory, storage, and networking considerations.

### Minimum Memory and CPU Requirements

The following table outlines the approximate minimum hardware requirements for different node roles:

| Role | Memory (RAM) | Virtual CPU (vCPU) |
| ----- | ----- | ----- |
| Controller Node | 1 GB | 1 vCPU |
| Worker Node | 0.5 GB | 1 vCPU |
| Controller \+ Worker | 1 GB | 1 vCPU |

These values are approximations; actual requirements may vary based on workload and cluster size.

### Controller Node Recommendations for Larger Clusters

For larger clusters, the recommended resources for controller nodes are:

| Number of Worker Nodes | Number of Pods | Recommended RAM | Recommended vCPU |
| ----- | ----- | ----- | ----- |
| Up to 10 | Up to 1,000 | 1–2 GB | 1–2 vCPU |
| Up to 50 | Up to 5,000 | 2–4 GB | 2–4 vCPU |
| Up to 100 | Up to 10,000 | 4–8 GB | 2–4 vCPU |
| Up to 500 | Up to 50,000 | 8–16 GB | 4–8 vCPU |
| Up to 1,000 | Up to 100,000 | 16–32 GB | 8–16 vCPU |
| Up to 5,000 | Up to 150,000 | 32–64 GB | 16–32 vCPU |

These recommendations help ensure optimal performance and stability for larger deployments.

### Storage Requirements

* **Controller Node**: Approximately 0.5 GB for k0s components; minimum 0.5 GB required.
* **Worker Node**: Approximately 1.3 GB for k0s components; minimum 1.6 GB required.
* **Controller \+ Worker**: Approximately 1.7 GB for k0s components; minimum 2.0 GB required.

It's recommended to use SSDs for optimal storage performance, as cluster latency and throughput are sensitive to storage performance.

### Networking Requirements

k0s requires certain network ports to be open for proper communication between components. Detailed information on the required ports and protocols can be found in the k0s networking documentation.

### Host Operating System and Architecture

* **Operating Systems**:
  * Linux (kernel version 4.3 or later)
    * x86-64
    * aarch64
    * armv7l
  * Windows Server 2019 (experimental)
    * x86-64

**Note:** k0s is actively tested on armv7 architecture which upstream Kubernetes does not currently do.
That already allowed us to identify and fix some architecture related issues that haven't been caught upstream, before they hit a stable Kubernetes release.

These specifications ensure compatibility across a wide range of hardware platforms.

### Additional Considerations

* **Operating System Dependencies**: k0s strives to be as independent from the OS as possible.
* The necessary kernel configurations and any external runtime dependencies are documented in the [k0s system requirements].

[k0s system requirements]: https://docs.k0sproject.io/stable/system-requirements/

## Storage requirements

### Controller Node Storage

* **Storage for Kubernetes Control Plane**:
  * Approximately **0.5 GB** is required for k0s system components on a controller-only node.
  * Persistent storage is essential for the **etcd data store**, which maintains the cluster’s state.
  * Recommended Storage:
    * **SSD** for improved etcd performance.
    * At least **20 GB** of free space for larger clusters with significant cluster state changes.

### Worker Node Storage

* **Storage for Kubernetes Workloads**:
  * Approximately **1.3 GB** is required for k0s system components.
  * Additional space is needed for container images, temporary files, and any application-specific storage.

## API Design

1. **Kubernetes API Compatibility**
   * k0s runs the upstream Kubernetes API server as-is, providing users with the standard Kubernetes API experience.
   * This ensures seamless interaction with Kubernetes-native tools (e.g., `kubectl`, Helm, and CI/CD systems) and compatibility with Kubernetes custom resources.
2. **Declarative API Model**
   * Like Kubernetes, k0s uses a declarative API model, enabling users to define desired states for resources (e.g., deployments, services, and custom objects).
   * The system continuously reconciles actual states with desired states, ensuring consistent and predictable behavior.
3. **API Evolution and Versioning**
   * The k0s API design follows Kubernetes’ API versioning practices, supporting multiple API versions (e.g., v1beta1, v1) for gradual transitions and backward compatibility.
     * We currently support only v1beta1 but are planning for the next version (v2)
   * Deprecated APIs are phased out according to Kubernetes release cycles, ensuring compatibility with upstream developments.

## Release process

k0s project follows closely the upstream Kubernetes release cycle. The only difference in the upstream Kubernetes release/maintenance schedule is that our initial release date is always a few weeks behind the upstream Kubernetes version release date as we are building our version of k0s from the officially released version of Kubernetes and need time for testing the final version before shipping.
![][image1]
The k0s version string consists of the Kubernetes version and the k0s version. For example:

```text
v1.32.6+k0s.1
```

The Kubernetes version (1.32.6) is the first part, and the last part (k0s.1) reflects the k0s version, which is built on top of the certain Kubernetes version.

## Installation

Here’s an example of how to set a single node cluster:

```console
# export K0S_VERSION=v1.32.6+k0s.1
# curl -sSfL https://github.com/k0sproject/k0s/releases/download/$K0S_VERSION/k0s-$K0S_VERSION-amd64 -o k0s
# chmod u+x k0s
# ./k0s install controller --single && ./k0s start

```

Naturally, this will spin up k0s with the default configuration. In case the user needs to configure something, they can create a yaml document to describe the [configuration](https://docs.k0sproject.io/stable/configuration/):

As part of the startup sequence, k0s performs a series of pre-flight checks. This ensures that the system meets the requirements,
such as all required kernel modules are loaded, enough free disk capacity on the node, and so on.
If the pre-flight checks fail, k0s will refuse to start unless it's explicitly told to, and the logs will clearly indicate why they failed.

Additionally, users have the option to run the CNCF Certified Kubernetes Conformance test suite.
This test suite is executed by the CI for each release (on amd64 and arm64) and its results are made available as release artifacts.

## Security

### CNCF Security self-assessment

See separate [document](security-self-assessment.md).

### Security hygiene

#### Code Quality and Development Practices

* **Version Control and Workflow**:
  * All development is managed through Git and a structured branching strategy.
  * Pull Requests (PRs) are mandatory for all changes, requiring reviews and approval by maintainers before merging.
* **Code Reviews**:
  * Every PR undergoes peer review to ensure adherence to coding standards and to identify potential issues early.
* **Automated Testing**:
  * **Unit Tests**: Validate individual components for functionality and correctness.
  * **End-to-End (E2E) Tests**: Assess cluster-level behaviors and workflows under real-world scenarios.
  * **Conformance Tests**: Ensure alignment with upstream Kubernetes standards.
* **Continuous Integration/Continuous Deployment (CI/CD)**:
  * CI pipelines run automated tests, perform builds, and validate compatibility before code is merged.
    * All CI is built on GitHub actions with declarative and open models

#### Security Practices

* **Dependency Management**:
  * **Dependabot** is used to automate security updates for vulnerable libraries.
  * **System Images** shipped with k0s are regurarly scanned by Trivy.
* **Signed Binaries**:
  * All k0s binaries are signed using **Cosign** as part of the release process to prevent tampering.
  * [Documentation](https://docs.k0sproject.io/stable/verifying-signs/) has instructions for users on how to verify the signature
* **Secure Default Configurations**:
  * k0s ships with secure-by-default settings, such as TLS encryption for API communication and RBAC enabled by default.
* **Vulnerability Response**:
  * A defined process for responding to discovered vulnerabilities, including prompt patching and communication with users.
    See [k0s security policy] for more information.

[k0s security policy]: https://github.com/k0sproject/k0s/blob/main/SECURITY.md

#### Community and Governance

* **Community Involvement**:
  * Security and health are bolstered by a transparent development process that invites contributions from the community.
  * Security issues can be reported confidentially to maintainers via a designated vulnerability disclosure program.
* **Governance Model**:
  * Decisions about features, security, and releases are guided by a structured governance framework involving maintainers and key contributors.
* **Documentation Standards**:
  * Comprehensive and updated documentation ensures users follow best practices, reducing misconfigurations and security risks.

#### Release Management

* **Semantic Versioning**:
  * Releases follow semantic versioning to provide clarity on the nature of changes (major, minor, or patch).
* **Testing Before Release**:
  * Each release undergoes rigorous automated testing to ensure stability and security.
* **Timely Security Patches**:
  * Critical vulnerabilities are addressed through prompt patch releases , given that an upstream fix is available.
* **Signed Binaries**:
  * k0s binaries are signed with **Cosign**, ensuring artifact integrity and authenticity.
* **Release Notes**:
  * Detailed notes accompany each release, highlighting new features, bug fixes, and security updates.

#### Compliance with CNCF Best Practices

* **Kubernetes Conformance**:
  * Regular conformance testing ensures compatibility with upstream Kubernetes APIs.
* **TAG Security Alignment**:
  * Security practices align with CNCF TAG Security recommendations, providing a robust baseline for health and security.

#### Supply Chain Security

* **Source Code Integrity**:
  * Code hosted on trusted platform on Github.
  * Git used as version control and all changes going through pull request process to ensure integrity.
* **Build Pipeline Security**:
  * All build pipelines are managed as-code (GH actions yaml) under project repository in GitHub.
  * All changes are going through normal review process.
  * Non-maintainer PRs require approval to run the CI workflows allowing a maintainer to verify the PR before allowing it to run.
* **Dependency Transparency**:
  * The project maintains transparency about its dependencies and their versions.
  * Automated dependency updates are handled via **Dependabot**.
  * A signed SBOM report, in spdx format, is produced for all releases.
