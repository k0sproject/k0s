# Run k0s in Docker

You can create a k0s cluster on top of docker. In such a scenario, by default, both controller and worker nodes are run in the same container to provide an easy local testing "cluster".

## Prerequisites

You will require a [Docker environment](https://docs.docker.com/get-docker/) running on a Mac, Windows, or Linux system.

## Container images

The k0s containers are published both on Docker Hub and GitHub. For reasons of simplicity, the examples given here use Docker Hub (GitHub requires a separate authentication that is not covered). Alternative links include:

- docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0
- ghcr.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0

**Note:** Due to Docker Hub tag validation scheme, we have to use `-` as the k0s version separator instead of the usual `+`. So for example k0s version `v{{{ extra.k8s_version }}}+k0s.0` is tagged as `docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0`.

## Start k0s

### 1. Initiate k0s

You can run your own k0s in Docker:

```sh
docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 --cgroupns=host docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0 -- k0s controller --enable-worker
```

Flags:
-d: Runs the container in detached mode.
--name k0s: Names the container k0s.
--hostname k0s: Sets the hostname of the container to k0s.
--privileged: Grants the container elevated privileges, required by k0s to function properly inside Docker.
-v /var/lib/k0s: Uses Docker volume. Mounts the docker container’s /var/lib/k0s directory to the host, ensuring that cluster data persists across container restarts.
-p 6443:6443: Exposes port 6443 on the host for the Kubernetes API server, allowing you to interact with the cluster externally.
--cgroupns=host: Configures the container to share the host's cgroup namespace, allowing k0s to monitor system resources accurately.
-- k0s controller --enable-worker: Starts the k0s controller and enables a worker node within the same container, creating a single-node cluster.

**Note:** This command starts k0s with a worker. You may disable the worker by running it without the flag `--enable-worker`

### 2. (Optional) Create additional workers

You can attach multiple workers nodes into the cluster to then distribute your application containers to separate workers.

For each required worker:

1. Acquire a join token for the worker:

    ```sh
    token=$(docker exec -t -i k0s k0s token create --role=worker)
    ```

2. Run the container to create and join the new worker:

    ```sh
    docker run -d --name k0s-worker1 --hostname k0s-worker1 --privileged -v /var/lib/k0s --cgroupns=host  docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0 k0s worker $token
    ```

Repeat these steps for each additional worker node needed. Ensure that workers can reach the controller on port 6443.

### 3. Access your cluster

a) Using kubectl within the Container

To check cluster status and list nodes, use:

```sh
docker exec k0s k0s kubectl get nodes
```

b) Using kubectl Locally

To configure local access to your k0s cluster, follow these steps:

1. Generate the kubeconfig:

    ```sh
    docker exec k0s k0s kubeconfig admin > ~/.kube/k0s.config
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

c) Use [Lens](https://github.com/lensapp/lens/):

Access the k0s cluster using Lens by following the instructions [here](https://docs.k8slens.dev/getting-started/add-cluster/).

## Use Docker Compose (alternative)

As an alternative you can run k0s using Docker Compose:

```yaml
version: "3.9"
services:
  k0s:
    container_name: k0s
    image: docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0
    command: k0s controller --config=/etc/k0s/config.yaml --enable-worker
    hostname: k0s
    privileged: true
    cgroup: host
    volumes:
      - "/var/lib/k0s"
    ports:
      - "6443:6443"
    network_mode: "bridge"
    environment:
      K0S_CONFIG: |-
        apiVersion: k0s.k0sproject.io/v1beta1
        kind: ClusterConfig
        metadata:
          name: k0s
        # Any additional configuration goes here ...
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
