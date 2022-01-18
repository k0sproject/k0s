module github.com/k0sproject/k0s

go 1.16

require (
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Microsoft/hcsshim v0.8.23
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535
	github.com/avast/retry-go v3.0.0+incompatible
	github.com/bombsimon/logrusr/v2 v2.0.1
	github.com/cloudflare/cfssl v1.4.1
	github.com/containerd/containerd v1.5.9
	github.com/davecgh/go-spew v1.1.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/libnetwork v0.8.0-dev.2.0.20201031180254-535ef365dc1d
	github.com/estesp/manifest-tool/v2 v2.0.0-beta.1
	github.com/evanphx/json-patch v4.12.0+incompatible
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/imdario/mergo v0.3.12
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/k0sproject/dig v0.2.0
	github.com/k0sproject/k0sctl v0.12.3
	github.com/kardianos/service v1.2.1-0.20210728001519-a323c3813bc7
	github.com/mitchellh/go-homedir v1.1.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/opencontainers/image-spec v1.0.2
	github.com/opencontainers/selinux v1.8.4 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rqlite/rqlite v4.6.0+incompatible
	github.com/segmentio/analytics-go v3.1.0+incompatible
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.4
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/weaveworks/footloose v0.0.0-20200609124411-8f3df89ea188
	github.com/zcalusic/sysinfo v0.0.0-20210905121133-6fa2f969a900
	go.etcd.io/etcd/api/v3 v3.5.1
	go.etcd.io/etcd/client/pkg/v3 v3.5.1
	go.etcd.io/etcd/client/v3 v3.5.1
	go.etcd.io/etcd/etcdutl/v3 v3.5.1
	go.uber.org/zap v1.19.1
	golang.org/x/crypto v0.0.0-20210817164053-32db794688a5
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20211029165221-6e7872819dc8
	google.golang.org/grpc v1.40.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.7.0
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/cli-runtime v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/cloud-provider v0.23.0
	k8s.io/component-base v0.23.0
	k8s.io/cri-api v0.23.0
	k8s.io/kube-aggregator v0.23.0
	k8s.io/kubectl v0.23.0
	k8s.io/mount-utils v0.23.0
	k8s.io/system-validators v1.4.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/controller-runtime v0.11.0
	sigs.k8s.io/yaml v1.3.0
)

// backported from k8s upstream, as a project which uses etcd, containerd and grpc at the same time, they have already selected versions which don't provide any interface compile time errors
replace (
	github.com/containerd/continuity => github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc
	github.com/containerd/ttrpc => github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl => github.com/containerd/typeurl v1.0.1
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.2+incompatible

	// make sure we don't have CVE-2020-28852
	golang.org/x/text => golang.org/x/text v0.3.6
)
