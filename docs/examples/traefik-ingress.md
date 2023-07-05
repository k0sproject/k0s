# Installing Traefik Ingress Controller

You can configure k0s with the [Traefik ingress controller](https://doc.traefik.io/traefik/providers/kubernetes-ingress/), a [MetalLB service loadbalancer](https://metallb.universe.tf/), and deploy the Traefik Dashboard using a service sample. To do this you leverage Helm's extensible bootstrapping functionality to add the correct extensions to the `k0s.yaml` file during cluster configuration.

## 1. Configure k0s.yaml

Configure k0s to install Traefik and MetalLB during cluster bootstrapping by adding their [Helm charts](../helm-charts.md) as extensions in the k0s configuration file (`k0s.yaml`).

**Note:**

A good practice is to have a small range of IP addresses that are addressable on your network, preferably outside the assignment pool your DHCP server allocates (though any valid IP range should work locally on your machine). Providing an addressable range allows you to access your load balancer and Ingress services from anywhere on your local network.

```yaml
extensions:
  helm:
    repositories:
    - name: traefik
      url: https://traefik.github.io/charts
    - name: bitnami
      url: https://charts.bitnami.com/bitnami
    charts:
    - name: traefik
      chartname: traefik/traefik
      version: "20.5.3"
      namespace: default
    - name: metallb
      chartname: bitnami/metallb
      version: "2.5.4"
      namespace: default
      values: |
        configInline:
          address-pools:
          - name: generic-cluster-pool
            protocol: layer2
            addresses:
            - 192.168.0.5-192.168.0.10
```

## 2. Retrieve the Load Balancer IP

After you start your cluster, run `kubectl get all` to confirm the deployment of Traefik and MetalLB. The command should return a response with the `metallb` and `traefik` resources, along with a service load balancer that has an assigned `EXTERNAL-IP`.

```shell
kubectl get all
```

*Output*:

```shell
NAME                                                 READY   STATUS    RESTARTS   AGE
pod/metallb-1607085578-controller-864c9757f6-bpx6r   1/1     Running   0          81s
pod/metallb-1607085578-speaker-245c2                 1/1     Running   0          60s
pod/traefik-1607085579-77bbc57699-b2f2t              1/1     Running   0          81s

NAME                         TYPE           CLUSTER-IP       EXTERNAL-IP      PORT(S)                      AGE
service/kubernetes           ClusterIP      10.96.0.1        <none>           443/TCP                      96s
service/traefik-1607085579   LoadBalancer   10.105.119.102   192.168.0.5      80:32153/TCP,443:30791/TCP   84s

NAME                                        DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR            AGE
daemonset.apps/metallb-1607085578-speaker   1         1         1       1            1           kubernetes.io/os=linux   87s

NAME                                            READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/metallb-1607085578-controller   1/1     1            1           87s
deployment.apps/traefik-1607085579              1/1     1            1           84s

NAME                                                       DESIRED   CURRENT   READY   AGE
replicaset.apps/metallb-1607085578-controller-864c9757f6   1         1         1       81s
replicaset.apps/traefik-1607085579-77bbc57699              1         1         1       81s
```

Take note of the `EXTERNAL-IP` given to the `service/traefik-n` load balancer. In this example, `192.168.0.5` has been assigned and can be used to access services via the Ingress proxy:

```shell
NAME                         TYPE           CLUSTER-IP       EXTERNAL-IP      PORT(S)                      AGE
service/traefik-1607085579   LoadBalancer   10.105.119.102   192.168.0.5      80:32153/TCP,443:30791/TCP   84s
```

Receiving a 404 response here is normal, as you've not configured any Ingress resources to respond yet:

```shell
curl http://192.168.0.5
```

```shell
404 page not found
```

## 3. Deploy and access the Traefik Dashboard

With an available and addressable load balancer present on your cluster, now you can quickly deploy the Traefik dashboard and access it from anywhere on your LAN (assuming that MetalLB is configured with an addressable range).

1. Create the Traefik Dashboard [IngressRoute](https://doc.traefik.io/traefik/providers/kubernetes-crd/) in a YAML file:

    ```yaml
    apiVersion: traefik.containo.us/v1alpha1
    kind: IngressRoute
    metadata:
      name: dashboard
    spec:
      entryPoints:
        - web
      routes:
        - match: PathPrefix(`/dashboard`) || PathPrefix(`/api`)
          kind: Rule
          services:
            - name: api@internal
              kind: TraefikService
    ```

2. Deploy the resource:

    ```shell
    kubectl apply -f traefik-dashboard.yaml
    ```

    *Output*:

    ```shell
    ingressroute.traefik.containo.us/dashboard created
    ```

    At this point you should be able to access the dashboard using the `EXTERNAL-IP` that you noted above by visiting `http://192.168.0.5/dashboard/` in your browser:

    ![Traefik Dashboard](../img/traefik-dashboard.png)

3. Create a simple `whoami` Deployment, Service, and [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/) manifest:

    ```yaml
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      name: whoami-deployment
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: whoami
      template:
        metadata:
          labels:
            app: whoami
        spec:
          containers:
          - name: whoami-container
            image: containous/whoami
    ---
    apiVersion: v1
    kind: Service
    metadata:
      name: whoami-service
    spec:
      ports:
      - name: http
        targetPort: 80
        port: 80
      selector:
        app: whoami
    ---
    apiVersion: networking.k8s.io/v1
    kind: Ingress
    metadata:
      name: whoami-ingress
    spec:
      rules:
      - http:
          paths:
          - path: /whoami
            pathType: Exact
            backend:
              service:
                name: whoami-service
                port:
                  number: 80
    ```

4. Apply the manifests:

    ```shell
    kubectl apply -f whoami.yaml
    ```

    *Output*:

    ```shell
    deployment.apps/whoami-deployment created
    service/whoami-service created
    ingress.networking.k8s.io/whoami-ingress created
    ```

5. Test the ingress and service:

    ```shell
    curl http://192.168.0.5/whoami
    ```

    *Output*:

    ```shell
    Hostname: whoami-deployment-85bfbd48f-7l77c
    IP: 127.0.0.1
    IP: ::1
    IP: 10.244.214.198
    IP: fe80::b049:f8ff:fe77:3e64
    RemoteAddr: 10.244.214.196:34858
    GET /whoami HTTP/1.1
    Host: 192.168.0.5
    User-Agent: curl/7.68.0
    Accept: */*
    Accept-Encoding: gzip
    X-Forwarded-For: 192.168.0.82
    X-Forwarded-Host: 192.168.0.5
    X-Forwarded-Port: 80
    X-Forwarded-Proto: http
    X-Forwarded-Server: traefik-1607085579-77bbc57699-b2f2t
    X-Real-Ip: 192.168.0.82
    ```

## Further details

With the Traefik Ingress Controller it is possible to use 3rd party tools, such as [ngrok](https://ngrok.io), to go further and expose your load balancer to the world. In doing this you enable dynamic certificate provisioning through [Let's Encrypt](https://letsencrypt.org/), using either [cert-manager](https://cert-manager.io/docs/) or Traefik's own built-in [ACME provider](https://doc.traefik.io/traefik/v2.0/user-guides/crd-acme/).
