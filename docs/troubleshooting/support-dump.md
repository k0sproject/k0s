# Support Insight

In many cases, especially when looking for [commercial support], there's a need
to share the cluster state with others. While one could always grant access to
the live cluster, this is not always desirable or possible. For these
situations, use the work provided by [Troubleshoot].

The Troubleshoot tool can produce a dump of the cluster state for sharing. The
[`sbctl`] tool can expose the dump tarball as a Kubernetes API.

The following example shows how this works with k0s.

[commercial support]: ../commercial-support.md
[Troubleshoot]: https://troubleshoot.sh
[`sbctl`]: https://github.com/replicatedhq/sbctl

## Setting up

To gather all the needed data, install another tool called [`support-bundle`].
Download it from the [Troubleshoot releases page]. Make sure to select the
executable for the correct architecture.

[`support-bundle`]: https://troubleshoot.sh/docs/support-bundle/introduction/
[Troubleshoot releases page]: https://github.com/replicatedhq/troubleshoot/releases

## Creating support bundle

A Support Bundle needs to know what to collect and optionally, what to analyze. This is defined in a YAML file.

While data collection can be customized to meet specific needs, the following
reference configuration for k0s covers core elements such as:

- collecting info on the host
- collecting system component statuses from `kube-system` namespace
- checking health of Kubernetes API, Etcd etc. components
- collecting k0s logs
- checking status of firewalls, anti-virus etc. services which are known to interfere with Kubernetes

Because host-level information is required, the commands must be run directly on
the k0s nodes.

After setting up the [tooling](#setting-up), run the following command to
generate a support bundle:

```shell
support-bundle --kubeconfig /var/lib/k0s/pki/admin.conf https://docs.k0sproject.io/stable/support-bundle-<role>.yaml
```

Above `<role>` refers to either `controller` or `worker`. Different roles
require different information. When running a controller with `--enable-worker`
or `--single`, which also makes it a worker, capture a combined dump:

```shell
support-bundle --kubeconfig /var/lib/k0s/pki/admin.conf https://docs.k0sproject.io/stable/support-bundle-controller.yaml https://docs.k0sproject.io/stable/support-bundle-worker.yaml
```

Once the data has been collected, a file named
`support-bundle-<timestamp>.tar.gz` will be created. Share this file as needed.
