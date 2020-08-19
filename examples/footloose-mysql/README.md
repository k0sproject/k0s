# MKE in footloose with mysql

In this example we're gonna use footloose to simulate 2-node control plane setup using mysql as the storage.

**Note:** All the config files are more like examples, you WILL need to finetune them to fit your env.

## Prep work

1. Install docker
2. Get footloose from https://github.com/weaveworks/footloose
3. Build mke bin: Run `make build` at repo root

## Mysql

Let's run mysql in docker:
```
docker run --name mke-mysql -p 3306:3306 -e MYSQL_DATABASE=mke -e MYSQL_ROOT_PASSWORD=mkerocks -d mysql:latest
```

Let's see which IP address docker gave the container:
```
 docker inspect -f {{.NetworkSettings.Networks.bridge.IPAddress}} mke-mysql
```

Fix the mysql address in `mke.yaml` config file.

## MKE control plane

First we need to bootup the control plane nodes:
```
footloose create
```

This will start two nodes for us: `controller0` and `controller1`. There's a bind-mount of the whole repo so we easily get the compiled bin into the nodes.

### Controller0

`controller0` will be our "primary" controller node so let's bootstrap it first.

SSH into the node with:
```
footloose ssh root@controller0
```

Bootstrap the controlplane:
```
# cd /root/mke
# ./mke server
```

Yes, really. It's that easy. In less than a minute we'll have the control plane up-and-running.

### Get the join token for second controller

To be able to join a second controller into the cluster we'll need a "join token". In `controller0` run a command:
```
mke token create --role controller
```

This will output the token we'll need in the next step.

### Join controller1 into control plane

SSH into the node with:
```
footloose ssh root@controller1
```

Bootstrap the controlplane:
```
# cd /root/mke
# ./mke server --join-address https://<controller0 IP>:9443 "that_long_token"
```

And voilà, our second controller node is up-and-running in no time.




