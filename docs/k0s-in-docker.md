# Running k0s in Docker

In this tutorial you'll create a k0s cluster on top of docker. By default, both controller and worker are run in the same container to provide an easy local testing "cluster". The tutorial also shows how to add additional worker nodes to the cluster.

### Prerequisites

Docker environment on Mac, Windows or Linux. [Get Docker](https://docs.docker.com/get-docker/).

### Container images

The k0s containers are published both on Docker Hub and GitHub. The examples in this page show Docker Hub, because it's more simple to use. Using GitHub requires a separate authentication (not covered here). Alternative links:

- docker.io/k0sproject/k0s:latest
- docker.pkg.github.com/k0sproject/k0s/k0s:"version"

### Installation steps

#### 1. Start k0s

You can run your own k0s in Docker easily with:
```sh
docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 docker.io/k0sproject/k0s:latest
```

#### 2. Create additional workers (optional)

If you want to attach multiple workers nodes into the cluster you can then distribute your application containers to separate workers.

First, we need a join token for the worker:
```sh
token=$(docker exec -t -i k0s k0s token create --role=worker)
```

Then create and join a new worker by running the container with:
```sh
docker run -d --name k0s-worker1 --hostname k0s-worker1 --privileged -v /var/lib/k0s docker.io/k0sproject/k0s:latest k0s worker $token
```

Repeat for as many workers you need, and have resources for. :)

#### 3. Access your cluster

You can access your cluster with kubectl:
```sh
docker exec k0s kubectl get nodes
```

Alternatively, grab the kubeconfig file with `docker exec k0s cat /var/lib/k0s/pki/admin.conf` and paste it e.g. into [Lens](https://github.com/lensapp/lens/).

### Docker Compose (alternative)

You can also run k0s with Docker Compose:
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
        kind: Cluster
        metadata:
          name: k0s
        # Any additional configuration goes here ...
```

### Known limitations

#### No custom Docker networks

Currently, we cannot run k0s nodes if the containers are configured to use custom networks e.g. with `--net my-net`. This is caused by the fact that Docker sets up a custom DNS service within the network and that messes up CoreDNS. We know that there are some workarounds possible, but they are bit hackish. And on the other hand, running k0s cluster(s) in bridge network should not cause issues.

### Next Steps

- [Installing with k0sctl](k0sctl-install.md) for deploying and upgrading multi-node clusters with one command
- [Control plane configuration options](configuration.md) for example for networking and datastore configuration
- [Worker node configuration options](worker-node-config.md) for example for node labels and kubelet arguments
- [Support for cloud providers](cloud-providers.md) for example for load balancer or storage configuration
- [Installing the Traefik Ingress Controller](examples/traefik-ingress.md), a tutorial for ingress deployment
