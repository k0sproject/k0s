## k0s kubectl

kubectl controls the Kubernetes cluster manager

### Synopsis

kubectl controls the Kubernetes cluster manager.

 Find more information at: https://kubernetes.io/docs/reference/kubectl/overview/

```shell
k0s kubectl [flags]
```

### Options

```shell
      --add-dir-header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --as string                        Username to impersonate for the operation
      --as-group stringArray             Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string                 Default cache directory (default "/home/ubuntu/.kube/cache")
      --certificate-authority string     Path to a cert file for the certificate authority
      --client-certificate string        Path to a client certificate file for TLS
      --client-key string                Path to a client key file for TLS
      --cluster string                   The name of the kubeconfig cluster to use
      --context string                   The name of the kubeconfig context to use
  -h, --help                             help for kubectl
      --insecure-skip-tls-verify         If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string                Path to the kubeconfig file to use for CLI requests.
      --log-backtrace-at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log-dir string                   If non-empty, write log files in this directory
      --log-file string                  If non-empty, use this log file
      --log-file-max-size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --match-server-version             Require server version to match client version
  -n, --namespace string                 If present, the namespace scope for this CLI request
      --one-output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --password string                  Password for basic authentication to the API server
      --profile string                   Name of profile to capture. One of (none|cpu|heap|goroutine|threadcreate|block|mutex) (default "none")
      --profile-output string            Name of the file to write the profile to (default "profile.pprof")
      --request-timeout string           The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                    The address and port of the Kubernetes API server
      --skip-headers                     If true, avoid header prefixes in the log messages
      --skip-log-headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
      --tls-server-name string           Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                     Bearer token for authentication to the API server
      --user string                      The name of the kubeconfig user to use
      --username string                  Username for basic authentication to the API server
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
      --warnings-as-errors               Treat warnings received from the server as errors and exit with a non-zero exit code
```

### Options inherited from parent commands

```shell
      --data-dir string                Data Directory for k0s (default: /var/lib/k0s). DO NOT CHANGE for an existing setup, things will break!
      --debug                          Debug logging (default: false)
      --log-flush-frequency duration   Maximum number of seconds between log flushes (default 5s)
      --version version[=true]         Print version information and quit
```

### SEE ALSO

* [k0s](k0s.md) - k0s - Zero Friction Kubernetes
* [k0s kubectl annotate](k0s_kubectl_annotate.md) - Update the annotations on a resource
* [k0s kubectl api-resources](k0s_kubectl_api-resources.md) - Print the supported API resources on the server
* [k0s kubectl api-versions](k0s_kubectl_api-versions.md) - Print the supported API versions on the server, in the form of "group/version"
* [k0s kubectl apply](k0s_kubectl_apply.md) - Apply a configuration to a resource by file name or stdin
* [k0s kubectl attach](k0s_kubectl_attach.md) - Attach to a running container
* [k0s kubectl auth](k0s_kubectl_auth.md) - Inspect authorization
* [k0s kubectl autoscale](k0s_kubectl_autoscale.md) - Auto-scale a deployment, replica set, stateful set, or replication controller
* [k0s kubectl certificate](k0s_kubectl_certificate.md) - Modify certificate resources.
* [k0s kubectl cluster-info](k0s_kubectl_cluster-info.md) - Display cluster information
* [k0s kubectl completion](k0s_kubectl_completion.md) - Output shell completion code for the specified shell (bash or zsh)
* [k0s kubectl config](k0s_kubectl_config.md) - Modify kubeconfig files
* [k0s kubectl cordon](k0s_kubectl_cordon.md) - Mark node as unschedulable
* [k0s kubectl cp](k0s_kubectl_cp.md) - Copy files and directories to and from containers
* [k0s kubectl create](k0s_kubectl_create.md) - Create a resource from a file or from stdin
* [k0s kubectl debug](k0s_kubectl_debug.md) - Create debugging sessions for troubleshooting workloads and nodes
* [k0s kubectl delete](k0s_kubectl_delete.md) - Delete resources by file names, stdin, resources and names, or by resources and label selector
* [k0s kubectl describe](k0s_kubectl_describe.md) - Show details of a specific resource or group of resources
* [k0s kubectl diff](k0s_kubectl_diff.md) - Diff the live version against a would-be applied version
* [k0s kubectl drain](k0s_kubectl_drain.md) - Drain node in preparation for maintenance
* [k0s kubectl edit](k0s_kubectl_edit.md) - Edit a resource on the server
* [k0s kubectl exec](k0s_kubectl_exec.md) - Execute a command in a container
* [k0s kubectl explain](k0s_kubectl_explain.md) - Get documentation for a resource
* [k0s kubectl expose](k0s_kubectl_expose.md) - Take a replication controller, service, deployment or pod and expose it as a new Kubernetes service
* [k0s kubectl get](k0s_kubectl_get.md) - Display one or many resources
* [k0s kubectl kustomize](k0s_kubectl_kustomize.md) - Build a kustomization target from a directory or URL.
* [k0s kubectl label](k0s_kubectl_label.md) - Update the labels on a resource
* [k0s kubectl logs](k0s_kubectl_logs.md) - Print the logs for a container in a pod
* [k0s kubectl options](k0s_kubectl_options.md) - Print the list of flags inherited by all commands
* [k0s kubectl patch](k0s_kubectl_patch.md) - Update fields of a resource
* [k0s kubectl plugin](k0s_kubectl_plugin.md) - Provides utilities for interacting with plugins
* [k0s kubectl port-forward](k0s_kubectl_port-forward.md) - Forward one or more local ports to a pod
* [k0s kubectl proxy](k0s_kubectl_proxy.md) - Run a proxy to the Kubernetes API server
* [k0s kubectl replace](k0s_kubectl_replace.md) - Replace a resource by file name or stdin
* [k0s kubectl rollout](k0s_kubectl_rollout.md) - Manage the rollout of a resource
* [k0s kubectl run](k0s_kubectl_run.md) - Run a particular image on the cluster
* [k0s kubectl scale](k0s_kubectl_scale.md) - Set a new size for a deployment, replica set, or replication controller
* [k0s kubectl set](k0s_kubectl_set.md) - Set specific features on objects
* [k0s kubectl taint](k0s_kubectl_taint.md) - Update the taints on one or more nodes
* [k0s kubectl top](k0s_kubectl_top.md) - Display resource (CPU/memory) usage
* [k0s kubectl uncordon](k0s_kubectl_uncordon.md) - Mark node as schedulable
* [k0s kubectl version](k0s_kubectl_version.md) - Print the client and server version information
* [k0s kubectl wait](k0s_kubectl_wait.md) - Experimental: Wait for a specific condition on one or many resources
