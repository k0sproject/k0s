# k0s Self-Assessment

This document evaluates the security posture of the k0s project, identifying current practices and areas for improvement to ensure robust security measures. 
It refers to the [TAG Security Self Assessment](https://docs.k0sproject.io/stable/governance/cncf/security-self-assessment/) on the k0s project done earlier.

## Table of Contents

* [Metadata](#metadata)
   * [Security links](#security-links)
* [Overview](#overview)
   * [Background](#background)
   * [Actors](#actors)   
* [Goals](#goals)
* [Self-assessment use](#self-assessment-use)
* [Project compliance](#project-compliance)
* [Secure development practices](#secure-development-practices)
* [Communication channels](#communication-channels)
* [Security issue resolution](#security-issue-resolution)
* [Appendix](#appendix)

## Metadata

|                   |                                              |
| ----------------- | -------------------------------------------- |
| Assessment Stage  | In Progress                                  |
| Software          | <https://github.com/k0sproject/k0s>          |
| Security Provider | No                                           |
| Languages         | Golang                                       |
| SBOM              | <https://github.com/k0sproject/k0s/releases/download/v1.33.4%2Bk0s.0/spdx.json>      |

### Security links

| Document      | URL                                              |
| ------------- | ------------------------------------------------ |
| Security file | <https://github.com/k0sproject/k0s/blob/main/SECURITY.md>      |

## Overview

Description:

k0s is a lightweight, open-source Kubernetes distribution designed to simplify cluster setup and management. It offers an all-inclusive binary for streamlined operations with minimal dependencies.
Key Use Cases:

- Simplifying Kubernetes deployment for edge and production environments.
- Managing distributed workloads at scale with reduced operational overhead.
- Security Goals:

Secure defaults for deployment and operation.
Flexibility for various deployment scenarios while adhering to Kubernetes security best practices.

### Background

### Actors

#### Control Plane

- **Components**: API Server, Scheduler, Controller Manager.
- **Role**: Centralized management of the Kubernetes cluster, handling scheduling, API requests, and resource management.
- **Security Mechanisms**:
  - Mutual TLS is used to secure communication between control plane components.
  - Anonymous access is disabled by default.
  - Role-Based Access Control (RBAC) ensures that only authorized users or processes can interact with the API Server.
    - K0s creates minimal RBAC for the system components it manages
  - Control plane nodes can be physically or logically separated from worker nodes to reduce exposure to potential compromise.
    - This is achieved with [konnectivity](https://github.com/kubernetes-sigs/apiserver-network-proxy) and is enabled by default.
  - K0s creates and manages the CA and other needed certificates
    - Serving and client certs are rotated on each k0s restart (The serving and client certs are valid for 1 year per default. Hence upgrading, and thus restarting k0s, at least once a year will rotate the certs automatically.)


#### Data Store (etcd)

- **Components**: Distributed key-value store for cluster state and secrets.
- **Role**: Maintains the state of the cluster, including configuration and sensitive data such as secrets.
- **Security Mechanisms**:
     - Etcd API is only exposed on localhost on controller nodes
     - Access to etcd is restricted to authenticated and authorized control plane components and secured with mutual TLS

#### Worker Nodes

- **Components**: Kubelet, kube-proxy, and container runtimes.
- **Role**: Execute workloads and interact with the control plane for orchestration.
- **Security Mechanisms**:
   - Worker nodes can be isolated from each other through network policies, preventing direct communication between pods unless explicitly allowed.\
   - Pod Security Standards (optional) enforce restrictions on workload capabilities, reducing the risk of lateral movement in case of a compromise.
   - Kubelet APIs are configured with authentication.
   - Kubelet is configured with certificate rotation.
   - Worker nodes are joined using revocable bootstrap tokens which can be configured to be short-living too.

#### Networking Layer

- **Components**: Cluster networking via CNI plugins, kube-proxy, or Calico
- **Role**: Facilitate communication between pods, services, and external systems.
- **Security Mechanisms**:
Network segmentation and policy enforcement restrict traffic between pods, nodes, and external endpoints.

## Goals

Security goals for k0s can be summarized as follows:

- Secure-by-default configurations
K0s in its default configuration should provide a secure base-level configuration. That includes things like no anonymous authentication, TLS enabled everywhere, RBAC, etc.. While that does NOT include integrations with things like AppArmor and seccomp, those configurations are exposed to users. In general, k0s allows users to finetune pretty much any and all Kubernetes options.

- Minimized operational complexity
By minimizing the operational complexity for Kubernetes the users have better and more easy control of their security configurations.

- Compatibility with Kubernetes security best practices#
k0s adheres to the established security frameworks and guidelines provided by Kubernetes, such as RBAC, network policies, and encryption mechanisms. This ensures that organizations can seamlessly integrate k0s into their existing Kubernetes environments without compromising security standards.

## Self-assessment use
This document evaluates the security posture of k0s, identifies existing measures, and highlights areas for improvement. It serves as a reference for stakeholders and to advance k0s at the CNCF Sandbox level.


## Project compliance
k0s follows the [CIS Kubernetes Benchmark](https://docs.k0sproject.io/stable/cis_benchmark/) with documented exceptions.

## Secure development practices

### Development pipeline
- Code Reviews: All changes undergo peer review by project maintainers.

- Dependency Management: Regular automated vulnerability scanning and updates of dependencies

- CI/CD Security: Security checks integrated into CI/CD pipelines. For non-maintainer pull requests, we require approval to run CI which allows us to verify

- Signed-off commits: All commits are required to be signed-off.

## Communication channels

- **Internal**: Internal, between maintainers, communication is handled mostly in Kubernetes Slack [#k0s-dev](https://kubernetes.slack.com/archives/C07VAPJUECS) channel, and in Mirantis internal channels for Mirantis core maintainers.

- **Inbound**: [#k0s-users](https://kubernetes.slack.com/archives/C0809EA06QZ) in Kubernetes Slack and GitHGub issues. Stack Overflow has also a [k0s tag](https://stackoverflow.com/questions/tagged/k0s) for related questions.

- **External**: k0s does not currently have any mailing lists. There are Mirantis-operated social media accounts that are used for communicating things like new releases etc.. There’s also a [k0sproject Medium](https://medium.com/k0sproject) account which is used for blogs.

  
## Security issue resolution

### Issue Reporting
Vulnerabilities can be reported via the k0s Github project using Github's [private security vulnerability reporting](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability) feature.

### Incident Response
The k0s maintainers triage and resolve issues. Security patches and advisories are published as needed.

## Appendix

### Case studies
[List of known users](https://docs.k0sproject.io/head/adopters/) along with some case studies.
