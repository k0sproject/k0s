package component

const etcdYaml = `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    scheduler.alpha.kubernetes.io/critical-pod: ""
  labels:
    component: etcd
    tier: control-plane
  name: etcd
  namespace: kube-system
spec:
  priorityClassName: system-node-critical
  containers:
  - command:
    - etcd
    - --name=controller
    - --enable-v2=true
    - --data-dir=/var/lib/etcd
    - --initial-cluster=controller=http://localhost:2380
    image: k8s.gcr.io/etcd:3.4.4
    name: etcd
    volumeMounts:
    - mountPath: /var/lib/etcd
      name: etcd-data
  hostNetwork: true
  volumes:
  - hostPath:
      path: /var/lib/etcd
      type: DirectoryOrCreate
    name: etcd-data
`

type Etcd struct {
}

type EtcdConfig struct {
}

// Run runs etcd
func (e Etcd) Run() error {
	tw := TemplateWriter{
		Name:     "etcd",
		Template: etcdYaml,
		Data:     EtcdConfig{},
		Path:     "/etc/kubernetes/manifests/mke-etcd.yaml",
	}

	return tw.Write()
}

// Stop does nothing, etcd runtime is controlled by kubelet
func (e Etcd) Stop() error {
	return nil
}
