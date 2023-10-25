# k0s in bootloose with mysql

In this example we're gonna use bootloose to simulate 2-node control plane setup using mysql as the storage.

**Note:** All the config files are more like examples, you WILL need to finetune them to fit your env.

## Prep work

1. Install docker
2. Get bootloose from https://github.com/k0sproject/bootloose
3. Build k0s bin: Run `make build` at repo root

## Mysql

Let's run mysql in docker:

```shell
docker run --name k0s-mysql -p 3306:3306 -e MYSQL_DATABASE=k0s -e MYSQL_ROOT_PASSWORD=k0srocks -d mysql:latest
```

Let's see which IP address docker gave the container:

```shell
 docker inspect -f {{.NetworkSettings.Networks.bridge.IPAddress}} k0s-mysql
```

Fix the mysql address in `k0s.yaml` config file.

## k0s control plane

First we need to bootup the control plane nodes:

```shell
bootloose create
```

This will start two nodes for us: `controller0` and `controller1`. There's a bind-mount of the whole repo so we easily get the compiled bin into the nodes.

### Controller0

`controller0` will be our "primary" controller node so let's bootstrap it first.

SSH into the node with:

```shell
bootloose ssh root@controller0
```

Bootstrap the controlplane:

```shell
cd /root/k0s
./k0s controller
```

Yes, really. It's that easy. In less than a minute we'll have the control plane up-and-running.

### Get the join token for second controller

To be able to join a second controller into the cluster we'll need a "join token". In `controller0` run a command:

```shell
k0s token create --role controller
```

This will output the token we'll need in the next step.

### Join controller1 into control plane

SSH into the node with:

```shell
bootloose ssh root@controller1
```

Bootstrap the controlplane:

```shell
cd /root/k0s
./k0s controller --join-address https://<controller0 IP>:9443 "that_long_token"
```

And voilà, our second controller node is up-and-running in no time.
