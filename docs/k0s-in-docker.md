# k0s in Docker

We publish k0s container image for every release. By default we run both controller and worker in the same container to allow easy local test "cluster".

You can run your own k0s-in-docker easily with:
```sh
docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 docker.pkg.github.com/k0sproject/k0s/k0s:<version>
```
Just grab the kubeconfig with `docker exec k0s cat /var/lib/k0s/pki/admin.conf` and paste e.g. into [Lens](https://github.com/lensapp/lens/).

## Running workers

If you want to attach multiple workers nodes into the cluster you can run separate containers for workers.

First, we need a join token for the worker:
```sh
token=$(docker exec -t -i k0s k0s token create --role=worker)
```

Then join a new worker by running the container with:
```
docker run -d --name k0s-worker1 --hostname k0s-worker1 --privileged -v /var/lib/k0s docker.pkg.github.com/k0sproject/k0s/k0s:<version> k0s worker $token
```

Repeat for as many workers you need, and have resources for. :)

## Docker Compose

You can also run k0s with Docker Compose:
```yaml
version: "3.9"
services:
  k0s:
    container_name: k0s
    image: docker.pkg.github.com/k0sproject/k0s/k0s:<version>
    hostname: k0s
    privileged: true
    volumes:
      - "/var/lib/k0s"
    ports:
      - "6443:6443"
    network_mode: "bridge"
```

## Known limitations

### No custom Docker networks

Currently we cannot run k0s nodes if the containers are configured to use custom networks e.g. with `--net my-net`. This is caused by the fact that Docker sets up a custom DNS service within the network and that messes up CoreDNS. We know that there are some workarounds possible, but they are bit hackish. And on the other hand, running k0s cluster(s) in bridge network should not cause issues.
