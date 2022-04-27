# Pod Security Standards

Since PodSecurityPolicy gets deprecated and will be removed in v1.25, Kubernetes offers [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) – a new way to enhance cluster security.

To enable PSS in k0s you need to create admission controller config file:

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

Add kube-apiserver arguments to the k0s configuration

    ```yaml
    apiVersion: k0s.k0sproject.io/v1beta1
    kind: ClusterConfig
    spec:
      api:
        extraArgs:
          disable-admission-plugins: PodSecurityPolicy # if you want to disable PodSecurityPolicy admission controller, not required
          enable-admission-plugins: PodSecurity        # only for Kubernetes 1.22, since 1.23 it's enabled by default
          feature-gates: "PodSecurity=true"                # only for Kubernetes 1.22, since 1.23 it's enabled by default
          admission-control-config-file: /path/to/admission/control/config.yaml
    ```

And finally, install k0s with the PodSecurityPolicy component disabled.

    ```shell
    $ k0s install controller --disable-components="default-psp
    ```