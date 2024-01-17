# System components monitoring

Controller nodes [are isolated](architecture.md#control-plane) by default, which thus means that a cluster user cannot schedule workloads onto controller nodes.

k0s provides a mechanism to expose system components for monitoring. System component metrics can give a better look into what is happening inside them. Metrics are particularly useful for building dashboards and alerts.
You can read more about metrics for Kubernetes system components [here](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/).

**Note:** the mechanism is an opt-in feature, you can enable it on installation:

```shell
sudo k0s install controller --enable-metrics-scraper
```

Once enabled, a new set of objects will appear in the cluster:

```shell
❯ ~ kubectl get all -n k0s-system
NAME                                   READY   STATUS    RESTARTS   AGE
pod/k0s-pushgateway-6c5d8c54cf-bh8sb   1/1     Running   0          43h

NAME                      TYPE        CLUSTER-IP      EXTERNAL-IP   PORT(S)    AGE
service/k0s-pushgateway   ClusterIP   10.100.11.116   <none>        9091/TCP   43h

NAME                              READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/k0s-pushgateway   1/1     1            1           43h

NAME                                         DESIRED   CURRENT   READY   AGE
replicaset.apps/k0s-pushgateway-6c5d8c54cf   1         1         1       43h
```

That's not enough to start scraping these additional metrics. For Prometheus
Operator](https://prometheus-operator.dev/) based solutions, you can create a
`ServiceMonitor` for it like this:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: k0s
  namespace: k0s-system
spec:
  endpoints:
  - port: http
  selector:
    matchLabels:
      app: k0s-observability
      component: pushgateway
      k0s.k0sproject.io/stack: metrics
```

Note that it won't clear alerts like "KubeControllerManagerDown" or
"KubeSchedulerDown" as they are based on Prometheus' internal "up" metrics. But
you can get rid of these alerts by modifying them to detect a working component
like this:

absent(apiserver_audit_event_total{job="kube-scheduler"})

## Jobs

The list of components which is scrapped by k0s:

- kube-scheduler
- kube-controller-manager
- etcd
- kine

**Note:** kube-apiserver metrics are not scrapped since they are accessible via `kubernetes` endpoint within the cluster.

## Architecture

![k0s metrics exposure architecture](img/pushgateway.png)

k0s uses pushgateway with TTL to make it possible to detect issues with the metrics delivery. Default TTL is 2 minutes.
