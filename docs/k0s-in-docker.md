# Run k0s in Docker

You can create a k0s cluster on top of docker. In such a scenario, by default, both controller and worker nodes are run in the same container to provide an easy local testing "cluster".

## Prerequisites

You will require a [Docker environment](https://docs.docker.com/get-docker/) running on a Mac, Windows, or Linux system.

## Container images

The k0s containers are published both on Docker Hub and GitHub. For reasons of simplicity, the examples given here use Docker Hub (GitHub requires a separate authentication that is not covered). Alternative links include:

- docker.io/k0sproject/k0s:latest
- docker.pkg.github.com/k0sproject/k0s/k0s:"version"

**Note:** Due to Docker Hub tag validation scheme, we have to use `-` as the k0s version separator instead of the usual `+`. So for example k0s version `v{{{ extra.k8s_version }}}+k0s.0` is tagged as `docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0`.

## Start k0s

### 1. Initiate k0s

You can run your own k0s in Docker:

```sh
docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 docker.io/k0sproject/k0s:latest
```

**Note:** If you are using Docker Desktop as the runtime, starting from 4.3.0 version it's using cgroups v2 in the VM that runs the engine. This means you have to add some extra flags to the above command to get kubelet and containerd to properly work with cgroups v2:

```sh
--cgroupns=host -v /sys/fs/cgroup:/sys/fs/cgroup:rw
```

### 2. (Optional) Create additional workers

You can attach multiple workers nodes into the cluster to then distribute your application containers to separate workers.

For each required worker:

1. Acquire a join token for the worker:

    ```sh
    token=$(docker exec -t -i k0s k0s token create --role=worker)
    ```

2. Run the container to create and join the new worker:

    ```sh
    docker run -d --name k0s-worker1 --hostname k0s-worker1 --privileged -v /var/lib/k0s docker.io/k0sproject/k0s:latest k0s worker $token
    ```

### 3. Access your cluster

Access your cluster using kubectl:

```sh
docker exec k0s kubectl get nodes
```

Alternatively, grab the kubeconfig file with `docker exec k0s cat /var/lib/k0s/pki/admin.conf` and paste it into [Lens](https://github.com/lensapp/lens/).

## Use Docker Compose (alternative)

As an alternative you can run k0s using Docker Compose:

```yaml
version: "3.9"
services:
  k0s:
    container_name: k0s
    image: docker.io/k0sproject/k0s:latest
    command: k0s controller --config=/etc/k0s/config.yaml --enable-worker
    hostname: k0s
    privileged: true
    volumes:
      - "/var/lib/k0s"
    tmpfs:
      - /run
      - /var/run
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
