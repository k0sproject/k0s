package component

import (
	"path/filepath"
	"time"

	config "github.com/Mirantis/mke/pkg/apis/v1beta1"
	"github.com/Mirantis/mke/pkg/constant"
	k8sutil "github.com/Mirantis/mke/pkg/kubernetes"
	"github.com/Mirantis/mke/pkg/util"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

const coreDnsTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRole
metadata:
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system-coredns
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1beta1
kind: ClusterRoleBinding
metadata:
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  name: system-coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-coredns
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: kube-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health
        ready
        kubernetes {{ .ClusterDomain }} in-addr.arpa ip6.arpa {
          pods insecure
          ttl 30
          fallthrough in-addr.arpa ip6.arpa
        }
        prometheus :9153
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "CoreDNS"
spec:
  replicas: {{ .Replicas}}
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
      serviceAccountName: coredns
      tolerations:
        - key: "CriticalAddonsOnly"
          operator: "Exists"
        - key: "node-role.kubernetes.io/master"
          operator: "Exists"
          effect: "NoSchedule"
      nodeSelector:
        beta.kubernetes.io/os: linux
      containers:
      - name: coredns
        image: docker.io/coredns/coredns:1.7.0
        imagePullPolicy: IfNotPresent
        resources:
          limits:
            memory: 170Mi
          requests:
            cpu: 100m
            memory: 70Mi
        args: [ "-conf", "/etc/coredns/Corefile" ]
        volumeMounts:
        - name: config-volume
          mountPath: /etc/coredns
          readOnly: true
        ports:
        - containerPort: 53
          name: dns
          protocol: UDP
        - containerPort: 53
          name: dns-tcp
          protocol: TCP
        - containerPort: 9153
          name: metrics
          protocol: TCP
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            add:
            - NET_BIND_SERVICE
            drop:
            - all
          readOnlyRootFilesystem: true
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 60
          periodSeconds: 10
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /ready
            port: 8181
            scheme: HTTP
          initialDelaySeconds: 0
          periodSeconds: 2
          timeoutSeconds: 1
          successThreshold: 1
          failureThreshold: 3
      dnsPolicy: Default
      volumes:
        - name: config-volume
          configMap:
            name: coredns
            items:
            - key: Corefile
              path: Corefile
---
apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  annotations:
    prometheus.io/port: "9153"
    prometheus.io/scrape: "true"
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "CoreDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: {{ .ClusterDNSIP }}
  ports:
  - name: dns
    port: 53
    protocol: UDP
  - name: dns-tcp
    port: 53
    protocol: TCP
  - name: metrics
    port: 9153
    protocol: TCP
`

type CoreDNS struct {
	client        *kubernetes.Clientset
	tickerDone    chan struct{}
	log           *logrus.Entry
	clusterConfig *config.ClusterSpec
}

type coreDNSConfig struct {
	Replicas      int
	ClusterDNSIP  string
	ClusterDomain string
}

func NewCoreDNS(clusterConfig *config.ClusterSpec) (*CoreDNS, error) {
	client, err := k8sutil.Client(constant.AdminKubeconfigConfigPath)
	if err != nil {
		return nil, err
	}
	log := logrus.WithFields(logrus.Fields{"component": "coredns"})
	return &CoreDNS{
		client:        client,
		log:           log,
		clusterConfig: clusterConfig,
	}, nil
}

func (c *CoreDNS) Init() error {
	return nil
}

func (c *CoreDNS) Run() error {

	c.tickerDone = make(chan struct{})

	// TODO calculate replicas, max-surge etc. based on amount of nodes

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		var previousConfig = coreDNSConfig{}
		for {
			select {
			case <-ticker.C:
				config, err := c.getConfig()
				if err != nil {
					c.log.Errorf("error calculating coredns configs: %s. will retry", err.Error)
					continue
				}
				if config == previousConfig {
					c.log.Infof("current config matches existing, not gonna do anything")
					continue
				}
				tw := util.TemplateWriter{
					Name:     "coredns",
					Template: coreDnsTemplate,
					Data:     config,
					Path:     filepath.Join(constant.DataDir, "manifests", "coredns.yaml"),
				}
				err = tw.Write()
				if err != nil {
					c.log.Errorf("error writing coredns manifests: %s. will retry", err.Error)
					continue
				}
				previousConfig = config
			case <-c.tickerDone:
				c.log.Info("coredns reconciler done")
				return
			}
		}
	}()

	return nil
}

func (c *CoreDNS) getConfig() (coreDNSConfig, error) {
	dns, err := c.clusterConfig.Network.DNSAddress()
	if err != nil {
		return coreDNSConfig{}, err
	}

	config := coreDNSConfig{
		Replicas:      1,
		ClusterDomain: "cluster.local",
		ClusterDNSIP:  dns,
	}

	return config, nil
}

func (c *CoreDNS) Stop() error {
	close(c.tickerDone)
	return nil
}
