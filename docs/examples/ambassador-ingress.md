# Installing the Ambassador Gateway on k0s

In this tutorial, you'll learn how to run k0s under Docker and configure it with the
[Ambassador API Gateway](https://www.getambassador.io/products/api-gateway/) and
a [MetalLB service loadbalancer](https://metallb.universe.tf/). We'll also deploy a sample 
service and expose it with an Ambassador mapping resource.

Utilizing the extensible bootstrapping functionality with Helm, 
it's as simple as adding the right extensions to the `k0s.yaml` file 
when configuring your cluster.

## Running k0s under docker

If you're not on a platform natively supported by k0s, running under docker is a viable option 
(see [k0s in Docker](../k0s-in-docker.md)). Since we're going to create a custom configuration file we'll need
to map that into the k0s container - and of course we'll need to expose the ports required by
Ambassador for outside access.

Start by running k0s under docker:

```sh
docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 docker.pkg.github.com/k0sproject/k0s/k0s:<version>
```

Once running export the default configuration file using

```sh
docker exec k0s k0s default-config > my-cluster.yaml
```

## Configuring k0s.yaml

Open the file in your favorite code editor and add the following extensions at the bottom:

```yaml
extensions:
  helm:
    repositories:
      - name: datawire
        url: https://www.getambassador.io
      - name: bitnami
        url: https://charts.bitnami.com/bitnami
    charts:
      - name: ambassador
        chartname: datawire/ambassador
        version: "6.5.13"
        namespace: ambassador
        values: |2
          service:
            externalIPs:
            - 172.17.0.2
      - name: metallb
        chartname: bitnami/metallb
        version: "1.0.1"
        namespace: default
        values: |2
          configInline:
            address-pools:
            - name: generic-cluster-pool
              protocol: layer2
              addresses:
              - 172.17.0.2
```

As you can see it adds both Ambassador and Metallb (required for LoadBalancers) with corresponding repositories
and (minimal) configurations. This example only uses your local network - providing a range of IPs for 
MetalLB that are addressable on your LAN is suggested if you want to access these services from anywhere on 
your network.

Now restart your k0s docker container with additional ports and the above config file mapped into it:

```sh
docker run --name k0s --hostname k0s --privileged -v /var/lib/k0s -v <path to conf file>:/k0s.yaml -p 6443:6443 -p 80:80 -p 443:443 -p 8080:8080 docker.pkg.github.com/k0sproject/k0s/k0s:v0.9.1
```

Let it start, and eventually you'll be able to list the Ambassador Services:

```shell
kubectl get services -n ambassador
NAME                          TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
ambassador-1611224811         LoadBalancer   10.99.84.151    172.17.0.2    80:30327/TCP,443:30355/TCP   2m11s
ambassador-1611224811-admin   ClusterIP      10.96.79.130    <none>        8877/TCP                     2m11s
ambassador-1611224811-redis   ClusterIP      10.110.33.229   <none>        6379/TCP                     2m11s
```

Install the Ambassador [edgectl tool](https://www.getambassador.io/docs/latest/topics/using/edgectl/edge-control/) 
and run the login command:

```shell
./edgectl login --namespace=ambassador localhost
```

This will open your browser and take you to the [Ambassador Console](https://www.getambassador.io/docs/latest/topics/using/edge-policy-console/) - all ready to go.

## Deploy / map a service

Let's deploy and map the [Swagger Petstore](https://petstore.swagger.io/) service; create a petstore.yaml file with
the following content.

```yaml
---
apiVersion: v1
kind: Service
metadata:
  name: petstore
  namespace: ambassador
spec:
  ports:
    - name: http
      port: 80
      targetPort: 8080
  selector:
    app: petstore
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: petstore
  namespace: ambassador
spec:
  replicas: 1
  selector:
    matchLabels:
      app: petstore
  strategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        app: petstore
    spec:
      containers:
        - name: petstore-backend
          image: docker.io/swaggerapi/petstore3:unstable
          ports:
            - name: http
              containerPort: 8080
---
apiVersion: getambassador.io/v2
kind:  Mapping
metadata:
  name: petstore
  namespace: ambassador
spec:
  prefix: /petstore/
  service: petstore
```

Once you've created this, apply it:

```sh
> kubectl apply -f petstore.yaml
service/petstore created
deployment.apps/petstore created
mapping.getambassador.io/petstore created
```

and you should be able to curl the service:

```shell
> curl -k https://localhost/petstore/api/v3/pet/findByStatus?status=available
[{"id":1,"category":{"id":2,"name":"Cats"},"name":"Cat 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":2,"category":{"id":2,"name":"Cats"},"name":"Cat 2","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag2"},{"id":2,"name":"tag3"}],"status":"available"},{"id":4,"category":{"id":1,"name":"Dogs"},"name":"Dog 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":7,"category":{"id":4,"name":"Lions"},"name":"Lion 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":8,"category":{"id":4,"name":"Lions"},"name":"Lion 2","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag2"},{"id":2,"name":"tag3"}],"status":"available"},{"id":9,"category":{"id":4,"name":"Lions"},"name":"Lion 3","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag3"},{"id":2,"name":"tag4"}],"status":"available"},{"id":10,"category":{"id":3,"name":"Rabbits"},"name":"Rabbit 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag3"},{"id":2,"name":"tag4"}],"status":"available"}]
```

or you can open https://localhost/petstore/ in your browser and change the URL of the specification to
https://localhost/petstore/api/v3/openapi.json (since we mapped it to the /petstore prefix). 

## Summary

This should get you all set with running Ambassador under k0s. If you're not running under Docker just skip the docker-related
steps above - but make sure that you have updated the k0s configuration in the same way as above. 