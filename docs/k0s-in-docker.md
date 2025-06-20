# Run k0s in Docker

You can create a k0s cluster on top of Docker.

## Prerequisites

You will require a [Docker environment](https://docs.docker.com/get-docker/) running on a Mac, Windows, or Linux system.

## Container images

The k0s OCI images are published to both Docker Hub and GitHub Container
registry. For simplicity, the examples given here use Docker Hub (GitHub
requires separate authentication, which is not covered here). The image names
are as follows:

- `docker.io/k0sproject/k0s:{{{ k0s_docker_version }}}`
- `ghcr.io/k0sproject/k0s:{{{ k0s_docker_version }}}`

**Note:** Due to Docker's tag validation scheme, `-` is used as the k0s version
separator instead of the usual `+`. For example, k0s version `{{{ k0s_version
}}}` is tagged as `docker.io/k0sproject/k0s:{{{ k0s_docker_version }}}`.

## Start k0s

### 1. Run a controller

By default, running the k0s OCI image will launch a controller with workloads
enabled (i.e. a controller with the `--enable-worker` flag) to provide an easy
local testing "cluster":

```sh
docker run -d --name k0s-controller --hostname k0s-controller \
  -v /var/lib/k0s -v /var/log/pods `# this is where k0s stores its data` \
  --tmpfs /run `# this is where k0s stores runtime data` \
  --privileged `# this is the easiest way to enable container-in-container workloads` \
  -p 6443:6443 `# publish the Kubernetes API server port` \
  docker.io/k0sproject/k0s:{{{ k0s_docker_version }}}
```

Explanation of command line arguments:

- `-d` runs the container in detached mode, i.e. in the background.
- `--name k0s-controller` names the container "k0s-controller".
- `--hostname k0s-controller` sets the hostname of the container to
  "k0s-controller".
- `-v /var/lib/k0s -v /var/log/pods` creates two Docker volumes and mounts them
  to `/var/lib/k0s` and `/var/log/pods` respectively inside the container,
  ensuring that cluster data persists across container restarts.
- `--tmpfs /run` **TODO**
- `--privileged` gives the container the elevated privileges that k0s needs to
  function properly within Docker. See the section on [adding additional
  workers](#2-optional-add-additional-workers) for a more detailed discussion of
  privileges.
- `-p 6443:6443` exposes the container's Kubernetes API server port 6443 to the
  host, allowing you to interact with the cluster externally.
- `docker.io/k0sproject/k0s:{{{ k0s_docker_version }}}` is the name of the
  k0s image to run.

By default, the k0s image starts a k0s controller with worker components enabled
within the same container, creating a cluster with a single
controller-and-worker node using the following command:

```Dockerfile
{% include "../Dockerfile" start="# Start CMD" end="# End CMD" %}
```

Alternatively, a controller-only node can be run like this:

```sh
docker run -d --name k0s-controller --hostname k0s-controller \
  --read-only `# k0s won't write any data outside the below paths` \
  -v /var/lib/k0s `# this is where k0s stores its data` \
  --tmpfs /run `# this is where k0s stores runtime data` \
  --tmpfs /tmp `# allow writing temporary files` \
  -p 6443:6443 `# publish the Kubernetes API server port` \
  docker.io/k0sproject/k0s:{{{ k0s_docker_version }}} \
  k0s controller
```

Note the addition of `k0s controller` to override the image's default command.
Also note that a controller-only node requires fewer privileges.

### 2. (Optional) Add additional workers

You can add multiple worker nodes to the cluster and then distribute your
application containers to separate workers.

1. Acquire a [join token](k0s-multi-node.md#3-create-a-join-token) for the
   worker:

   ```sh
   token=$(docker exec k0s-controller k0s token create --role=worker)
   ```

2. Run the container to create and join the new worker:

   ```sh
   docker run -d --name k0s-worker1 --hostname k0s-worker1 \
     -v /var/lib/k0s -v /var/log/pods `# this is where k0s stores its data` \
     --tmpfs /run `# this is where k0s stores runtime data` \
     --privileged `# this is the easiest way to enable container-in-container workloads` \
     docker.io/k0sproject/k0s:{{{ k0s_docker_version }}} \
     k0s worker $token
   ```

   Alternatively, with fine-grained privileges:
   <!--
     This setup is partly repeated in compose.yaml. So if things change here,
     they should probably be reflected in compose.yaml as well.

     Ideally, this example would show a setup with a read-only root file system.
     Unfortunately, the entrypoint's DNS fixup needs to modify /etc/resolv.conf,
     so this is not an option at this time. The entrypoint could perhaps try to
     overmount /etc/resolv.conf, but that stunt is left for the future.
     Additional paths that should then be added as tmpfs:
     - /tmp
     - /etc/cni/net.d
     - /opt/cni/bin
   -->

   ```sh
   docker run -d --name k0s-worker1 --hostname k0s-worker1 \
     -v /var/lib/k0s -v /var/log/pods `# this is where k0s stores its data` \
     --tmpfs /run `# this is where k0s stores runtime data` \
     --security-opt seccomp=unconfined \
     -v /dev/kmsg:/dev/kmsg:ro --device-cgroup-rule='c 1:11 r' \
     --cap-add sys_admin \
     --cap-add net_admin \
     --cap-add sys_ptrace \
     --cap-add sys_resource \
     docker.io/k0sproject/k0s:{{{ k0s_docker_version }}} \
     k0s worker "$token"
   ```

   Notes on the security-related flags:

   - `--security-opt seccomp=unconfined` is required for `runc` to access the
     [session keyring].
   - `-v /dev/kmsg:/dev/kmsg:ro --device-cgroup-rule='c 1:11 r'` allows reading
     of `/dev/kmsg` from inside the container. The kubelet's OOM watcher uses
     this.
     <!-- check device type via `stat -c %Hr:%Lr /dev/kmsg`. -->
     <!--
       Upstream reference: https://github.com/euank/go-kmsg-parser/blob/v2.0.0/kmsgparser/kmsgparser.go#L60
       Also relevant: KubeletInUserNamespace feature gate (alpha since v1.22)
       https://kubernetes.io/docs/tasks/administer-cluster/kubelet-in-userns/
     -->

   Notes on [Linux capabilities]:

   - `CAP_SYS_ADMIN` allows for a variety of administrative tasks, including
     mounting file systems and managing namespaces, which are necessary for
     creating and configuring nested containers.
   - `CAP_NET_ADMIN` allows manipulation of network settings such as interfaces
     and routes, allowing containers to create isolated or bridged networks, and
     so on.
   - `CAP_SYS_PTRACE` allows to inspect and modify processes, used to monitor
     other containers in a nested environment.
     <!--
       Omitting this results in not being able to start containers
       ("RunContainerError")
     -->
   - `CAP_SYS_RESOURCE` allows containers to override resource limits for things
     like memory or file descriptors, used to manage and adjust resource
     allocation in nested container environments.
     <!--
       Omitting this results in "runc create failed: unable to start container
       process: can't get final child's PID from pipe: EOF: unknown"
     -->

   Note that more privileges may be required depending on your cluster
   configuration and workloads.

   Repeat this step for each additional worker node and adjust the container and
   host names accordingly. Make sure that the workers can reach the controller
   on the [required ports]. If you are using Docker's default bridged network,
   this should be the case.

[session keyring]: https://www.man7.org/linux/man-pages/man7/session-keyring.7.html
[Linux capabilities]: https://www.man7.org/linux/man-pages/man7/capabilities.7.html
[required ports]: networking.md#controller-worker-communication

### 3. Access your cluster

#### a) Using kubectl within the container

To check cluster status and list nodes, use:

```sh
docker exec k0s-controller k0s kubectl get nodes
```

#### b) Using kubectl locally

To configure local access to your k0s cluster, follow these steps:

1. Generate the kubeconfig:

    ```sh
    docker exec k0s-controller k0s kubeconfig admin > ~/.kube/k0s.config
    ```

2. Update kubeconfig with Localhost Access:

    To automatically replace the server IP with localhost dynamically in `~/.kube/k0s.config`, use the following command:

    ```sh
    sed -i '' -e "$(awk '/server:/ {print NR; exit}' ~/.kube/k0s.config)s|https://.*:6443|https://localhost:6443|" ~/.kube/k0s.config
    ```

    This command updates the kubeconfig to point to localhost, allowing access to the API server from your host machine

3. Set the KUBECONFIG Environment Variable:

    ```sh
    export KUBECONFIG=~/.kube/k0s.config
    ```

4. Verify Cluster Access:

    ```sh
    kubectl get nodes
    ```

#### c) Use [Lens]

Access the k0s cluster using Lens by following the instructions on [how to add a
cluster].

[Lens]: https://k8slens.dev/
[how to add a cluster]: https://docs.k8slens.dev/getting-started/add-cluster/

## Use Docker Compose (alternative)

As an alternative you can run k0s using Docker Compose:

<!-- Kept in its own file to ease local testing. -->
```yaml
{% include "compose.yaml" %}
```

## Known limitations

### No custom Docker networks

Currently, k0s nodes cannot be run if the containers are configured to use custom networks (for example, with `--net my-net`). This is because Docker sets up a custom DNS service within the network which creates issues with CoreDNS. No completely reliable workaounds are available, however no issues should arise from running k0s cluster(s) on a bridge network.

## Next Steps

- [Install using k0sctl](k0sctl-install.md): Deploy multi-node clusters using just one command
- [Control plane configuration options](configuration.md): Networking and datastore configuration
- [Worker node configuration options](worker-node-config.md): Node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md): Load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md): Ingress deployment information
