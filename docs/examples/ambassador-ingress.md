# Installing Ambassador API Gateway

You can configure k0s with the [Ambassador API Gateway](https://www.getambassador.io/products/api-gateway/) and a [MetalLB service loadbalancer](https://metallb.universe.tf/). To do this you leverage Helm's extensible bootstrapping functionality to add the correct extensions to the `k0s.yaml` file during cluster configuration.

Note: Currently Ambassador API Gateway does not support Kubernetes v1.22 or above. See [here](https://github.com/emissary-ingress/emissary/issues/3735#issuecomment-916278895) for details.

## Use Docker for non-native k0s platforms

With Docker you can run k0s on platforms that the distribution does not natively support (refer to [Run k0s in Docker](../k0s-in-docker.md)). Skip this section if you are on a platform that k0s natively supports.

As you need to create a custom configuration file to install Ambassador Gateway, you will first need to map that file into the k0s container and to expose the ports Ambassador needs for outside access.

1. Run k0s under Docker:

    ```shell
    docker run -d --name k0s --hostname k0s --privileged -v /var/lib/k0s -p 6443:6443 docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0
    ```

2. Export the default k0s configuration file:

    ```shell
    docker exec k0s k0s config create > k0s.yaml
    ```

3. Export the cluster config, so you can access it using kubectl:

    ```shell
    docker exec k0s cat /var/lib/k0s/pki/admin.conf > k0s-cluster.conf
    export KUBECONFIG="$KUBECONFIG:$PWD/k0s-cluster.conf"
    ```

## Configure `k0s.yaml` for Ambassador Gateway

1. Open the `k0s.yml` file and append the following extensions at the end:

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

    **Note**: It may be necessary to replace the 172.17.0.2 IP with your local IP address.

    This action adds both Ambassador and MetalLB (required for LoadBalancers) with the corresponding repositories and (minimal) configurations. Be aware that the provided example illustrates the use of your local network and that you will want to provide a range of IPs for MetalLB that are addressable on your LAN to access these services from anywhere on your network.

2. Stop/remove your k0s container:

    ```shell
    docker stop k0s
    docker rm k0s
    ```

3. Retart your k0s container, this time with additional ports and the above config file mapped into it:

    ```shell
    docker run --name k0s --hostname k0s --privileged -v /var/lib/k0s -v "$PWD"/k0s.yaml:/k0s.yaml -p 6443:6443 -p 80:80 -p 443:443 -p 8080:8080 docker.io/k0sproject/k0s:v{{{ extra.k8s_version }}}-k0s.0
    ```

    After some time, you will be able to list the Ambassador Services:

    ```shell
    kubectl get services -n ambassador
    ```

    *Output*:

    ```shell
    NAME                          TYPE           CLUSTER-IP      EXTERNAL-IP   PORT(S)                      AGE
    ambassador-1611224811         LoadBalancer   10.99.84.151    172.17.0.2    80:30327/TCP,443:30355/TCP   2m11s
    ambassador-1611224811-admin   ClusterIP      10.96.79.130    <none>        8877/TCP                     2m11s
    ambassador-1611224811-redis   ClusterIP      10.110.33.229   <none>        6379/TCP                     2m11s
    ```

4. Install the Ambassador [edgectl tool](https://www.getambassador.io/docs/latest/topics/using/edgectl/edge-control/) and run the login command:

    ```shell
    edgectl login --namespace=ambassador localhost
    ```

    Your browser will open and deeliver you to the [Ambassador Console](https://www.getambassador.io/docs/latest/topics/using/edge-policy-console/).

## Deploy / Map a Service

1. Create a YAML file for the service (for example purposes, create a [Swagger Petstore](https://petstore.swagger.io/) service using a petstore.YAML file):

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

2. Apply the YAML file:

    ```shell
    kubectl apply -f petstore.yaml
    ```

    *Output*:

    ```shell
    service/petstore created
    deployment.apps/petstore created
    mapping.getambassador.io/petstore created
    ```

3. Validate that the service is running.

    In the terminal using curl:

    ```shell
    curl -k 'https://localhost/petstore/api/v3/pet/findByStatus?status=available'
    ```

    *Output*:

    ```json
    [{"id":1,"category":{"id":2,"name":"Cats"},"name":"Cat 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":2,"category":{"id":2,"name":"Cats"},"name":"Cat 2","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag2"},{"id":2,"name":"tag3"}],"status":"available"},{"id":4,"category":{"id":1,"name":"Dogs"},"name":"Dog 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":7,"category":{"id":4,"name":"Lions"},"name":"Lion 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag1"},{"id":2,"name":"tag2"}],"status":"available"},{"id":8,"category":{"id":4,"name":"Lions"},"name":"Lion 2","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag2"},{"id":2,"name":"tag3"}],"status":"available"},{"id":9,"category":{"id":4,"name":"Lions"},"name":"Lion 3","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag3"},{"id":2,"name":"tag4"}],"status":"available"},{"id":10,"category":{"id":3,"name":"Rabbits"},"name":"Rabbit 1","photoUrls":["url1","url2"],"tags":[{"id":1,"name":"tag3"},{"id":2,"name":"tag4"}],"status":"available"}]
    ```

    Or by way of your browser:

    Open https://localhost/petstore/ in your browser and change the URL in the field at the top of the page to https://localhost/petstore/api/v3/openapi.json (as it is mapped to the /petstore prefix) and click **Explore**.

4. Navigate to the **Mappings** area in the Ambassador Console to view the corresponding PetStore mapping as configured.
