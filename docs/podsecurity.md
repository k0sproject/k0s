# Pod Security Standards

Since Pod Security Policies have been removed in Kubernetes v1.25, Kubernetes
offers [Pod Security Standards] – a new way to enhance cluster security.

To enable PSS in k0s you need to create an admission controller config file:

    ```yaml
    apiVersion: apiserver.config.k8s.io/v1
    kind: AdmissionConfiguration
    plugins:
    - name: PodSecurity
      configuration:
        apiVersion: pod-security.admission.config.k8s.io/v1beta1
        kind: PodSecurityConfiguration
        # Defaults applied when a mode label is not set.
        defaults:
          enforce: "privileged"
          enforce-version: "latest"
        exemptions:
          # Don't forget to exempt namespaces or users that are responsible for deploying
          # cluster components, because they need to run privileged containers
          usernames: ["admin"]
          namespaces: ["kube-system"]
    ```

Add these extra arguments to the k0s configuration:

    ```yaml
    apiVersion: k0s.k0sproject.io/v1beta1
    kind: ClusterConfig
    spec:
      api:
        extraArgs:
          admission-control-config-file: /path/to/admission/control/config.yaml
    ```

[Pod Security Standards]: https://kubernetes.io/docs/concepts/security/pod-security-standards/
