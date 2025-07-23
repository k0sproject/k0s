<!--
SPDX-FileCopyrightText: 2021 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Installing NGINX Ingress Controller

This tutorial covers the installation of [Ingress NGINX Controller], which is an
open source project made by the Kubernetes community. k0s doesn't come with a
built-in ingress controller, but it's easy to deploy Ingress NGINX Controller as
shown in this document. Other ingress solutions can be used as well (see the
links at the end of the page).

[Ingress NGINX Controller]: https://github.com/kubernetes/ingress-nginx

## `NodePort` vs. `LoadBalancer` vs. `Ingress`

Kubernetes offers multiple options for exposing services to external networks. The main options are `NodePort`, `LoadBalancer` and `Ingress`.

**NodePort**, as the name says, means that a port on a node is configured to route incoming requests to a certain service. The port range is limited to 30000-32767, so you cannot expose commonly used ports like 80 or 443 via `NodePort`.

**LoadBalancer** is a service, which is typically implemented by the cloud provider as an external service (with additional cost). Load balancers can also be installed internally in the Kubernetes cluster with MetalLB, which is typically used for bare-metal deployments. Load balancer provides a single IP address to access your services, which can run on multiple nodes.

**Ingress controller** helps to consolidate routing rules of multiple applications into one entity. Ingress controller is exposed to an external network with the help of `NodePort`, `LoadBalancer` or host network. You can also use Ingress controller to terminate TLS for your domain in one place, instead of terminating TLS for each application separately.

## Ingress NGINX Controller

Ingress NGINX Controller is a very popular ingress for Kubernetes. In many cloud environments, it can be exposed to an external network by using the load balancer offered by the cloud provider. However, cloud load balancers are not necessary. Load balancer can also be implemented with MetalLB, which can be deployed in the same Kubernetes cluster. Another option to expose the Ingress controller to an external network is to use `NodePort`. The third option is to use host network. All of these alternatives are described in more detail on below, with separate examples.

![k0s_ingress_controller](../img/k0s_ingress_controller.png)

### Installation using `NodePort`

Installing using `NodePort` is the most simple example for Ingress NGINX Controller as we can avoid the load balancer dependency. `NodePort` is used for exposing the controller to the external network.

1. Install Ingress NGINX Controller (using the official manifests from the official `ingress-nginx` Kubernetes git repository)

    ```shell
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.1.3/deploy/static/provider/baremetal/deploy.yaml
    ```

2. Check that the Ingress controller pods have started

    ```shell
    kubectl get pods -n ingress-nginx
    ```

3. Check that you can see the NodePort service

    ```shell
    kubectl get services -n ingress-nginx
    ```

4. From version `v1.0.0` of the Ingress NGINX Controller, [a ingressclass object is required](https://kubernetes.github.io/ingress-nginx/#what-is-an-ingressclass-and-why-is-it-important-for-users-of-ingress-nginx-controller-now).

    In the default installation, an ingressclass object named `nginx` has already been created.

    ```shell
    $ kubectl -n ingress-nginx get ingressclasses
    NAME    CONTROLLER             PARAMETERS   AGE
    nginx   k8s.io/ingress-nginx   <none>       162m
    ```

    If this is only instance of the Ingress NGINX Controller, you should [add the annotation](https://kubernetes.github.io/ingress-nginx/#i-have-only-one-instance-of-the-ingresss-nginx-controller-in-my-cluster-what-should-i-do) `ingressclass.kubernetes.io/is-default-class` in your ingress class:

    ```shell
    kubectl -n ingress-nginx annotate ingressclasses nginx ingressclass.kubernetes.io/is-default-class="true"
    ```

5. Try connecting the controller using the `NodePort` from the previous step (in the range of 30000-32767)

    ```shell
    curl <worker-external-ip>:<node-port>
    ```

    If you don't yet have any backend service configured, you should see "404 Not Found" from nginx. This is okay for now. If you see a response from nginx, the ingress controller is running and you can reach it.

6. Deploy a small test application (Apache HTTP Server) to verify your ingress controller.

    Create the following YAML file and name it `simple-web-server-with-ingress.yaml`:

    ```yaml
    apiVersion: v1
    kind: Namespace
    metadata:
      name: web
    ---
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: web-server
      namespace: web
    spec:
      selector:
        matchLabels:
          app: web
      template:
        metadata:
          labels:
            app: web
        spec:
          containers:
          - name: httpd
            image: httpd:2.4.53-alpine
            ports:
            - containerPort: 80
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: web-server-service
      namespace: web
    spec:
      selector:
        app: web
      ports:
        - protocol: TCP
          port: 5000
          targetPort: 80
    ---
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: web-server-ingress
      namespace: web
    spec:
      ingressClassName: nginx
      rules:
      - host: web.example.com
        http:
          paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web-server-service
                port:
                  number: 5000
    ```

    Deploy the app:

    ```shell
    kubectl apply -f simple-web-server-with-ingress.yaml
    ```

7. Verify that you can access your application using the `NodePort` from step 3.

    ```shell
    curl <worker-external-ip>:<node-port> -H 'Host: web.example.com'
    ```

    If you are successful, you should see ```<html><body><h1>It works!</h1></body></html>```.

### Install using `LoadBalancer`

In this example you'll install Ingress NGINX Controller using `LoadBalancer` on k0s.

1. Install a `LoadBalancer`

    There are two alternatives to install `LoadBalancer` on k0s. Follow the links in order to install `LoadBalancer`.

    - [MetalLB](metallb-loadbalancer.md) as a pure software solution running internally in the k0s cluster
    - [Cloud provider's](../cloud-providers.md) load balancer running outside of the k0s cluster

2. Verify `LoadBalancer`

    In order to proceed you need to have a load balancer available for the Kubernetes cluster. To verify that it's available, deploy a simple Service of type `LoadBalancer`.

    ```yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: example-load-balancer
    spec:
      selector:
        app: web
      ports:
        - protocol: TCP
          port: 80
          targetPort: 80
      type: LoadBalancer
    ```

    ```shell
    kubectl apply -f example-load-balancer.yaml
    ```

    Then run the following command to see your `LoadBalancer` with an external IP address.

    ```shell
    kubectl get service example-load-balancer
    ```

    If the `LoadBalancer` is not available, you won't get an IP address for EXTERNAL-IP. Instead, it's ```<pending>```. In this case you should go back to the previous step and check your load balancer availability.

    If you are successful, you'll see a real IP address and you can proceed further.

    You can delete the example-load-balancer:

    ```shell
    kubectl delete -f example-load-balancer.yaml
    ```

3. Install Ingress NGINX Controller by following the steps in the previous chapter (step 1 to step 4).

4. Edit the Ingress NGINX Controller to use `LoadBalancer` instead of `NodePort`

    ```shell
    kubectl edit service ingress-nginx-controller -n ingress-nginx
    ```

    Find the spec.type field and change it from `NodePort` to `LoadBalancer`.

5. Check that you can see the `ingress-nginx` service with type `LoadBalancer`.

    ```shell
    kubectl get services -n ingress-nginx
    ```

6. Try connecting to the ingress controller

    If you used private IP addresses for MetalLB in step 2, you should run the following command from the local network. Use the IP address from the previous step, column EXTERNAL-IP.

    ```shell
    curl <EXTERNAL-IP>
    ```

    If you don't yet have any backend service configured, you should see "404 Not Found" from nginx. This is okay for now. If you see a response from nginx, the Ingress Controller is running and you can reach it using `LoadBalancer`.

7. Deploy a small test application (Apache HTTP Server) to verify your `Ingress`.

    Create the YAML file `simple-web-server-with-ingress.yaml` as described in the previous chapter (step 6) and deploy it.

    ```shell
    kubectl apply -f simple-web-server-with-ingress.yaml
    ```

8. Verify that you can access your application through the `LoadBalancer` and ingress controller.

    ```shell
    curl <worker-external-ip> -H 'Host: web.example.com'
    ```

    If you are successful, you should see ```<html><body><h1>It works!</h1></body></html>```.

### Install using host network

The host network option exposes Ingress directly using the worker nodes' IP addresses. It also allows you to use ports 80 and 443. This option doesn't use any Service objects (`ClusterIP`, `NodePort`, `LoadBalancer`) and it has the limitation that only one Ingress controller Pod may be scheduled on each cluster node.

1. Install Ingress NGINX Controller manifests:

    ```shell
    wget https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.1.3/deploy/static/provider/baremetal/deploy.yaml
    ```

2. Edit `deploy.yaml`. Find the Deployment ingress-nginx-controller and enable the host network option by adding the `hostNetwork` line:

    ```shell
    spec:
      template:
        spec:
          hostNetwork: true
    ```

    You can also remove the Service ingress-nginx-controller completely, because it won't be needed.

3. Install Ingress

    ```shell
    kubectl apply -f deploy.yaml
    ```

4. Try to connect to the ingress controller, deploy a test application and verify the access. These steps are similar to the previous install methods.

## Additional information

For more information about Ingress NGINX Controller installation, take a look at
the official [installation guide] and [bare-metal considerations].

[installation guide]: https://kubernetes.github.io/ingress-nginx/deploy/
[bare-metal considerations]: https://kubernetes.github.io/ingress-nginx/deploy/baremetal/

## Alternative examples for ingress controllers on k0s

[Traefik Ingress](traefik-ingress.md)
