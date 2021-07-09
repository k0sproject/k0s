module github.com/k0sproject/k0s

go 1.16

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Microsoft/hcsshim v0.8.18
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cloudflare/cfssl v1.4.1
	github.com/containerd/containerd v1.5.4
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/libnetwork v0.5.6
	github.com/evanphx/json-patch v4.11.0+incompatible
	github.com/garyburd/redigo v1.6.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.5
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.11
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/k0sproject/dig v0.1.0
	github.com/kardianos/service v1.2.1-0.20210728001519-a323c3813bc7
	github.com/magiconair/properties v1.8.5
	github.com/mitchellh/go-homedir v1.1.0
	github.com/olekukonko/tablewriter v0.0.5
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.8.4 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	github.com/rqlite/rqlite v0.0.0-20210528155034-8dc8788f37db
	github.com/segmentio/analytics-go v3.1.0+incompatible
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.4
	github.com/vishvananda/netlink v1.1.1-0.20201029203352-d40f9887b852
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/weaveworks/footloose v0.0.0-20200609124411-8f3df89ea188
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/zcalusic/sysinfo v0.0.0-20210226105846-b810d137e525
	go.etcd.io/etcd/api/v3 v3.5.0
	go.etcd.io/etcd/client/pkg/v3 v3.5.0
	go.etcd.io/etcd/client/v3 v3.5.0
	go.etcd.io/etcd/etcdutl/v3 v3.5.0
	go.uber.org/zap v1.17.0
	golang.org/x/crypto v0.0.0-20210513164829-c07d793c2f9a
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.38.0
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.4.0
	helm.sh/helm/v3 v3.6.1-0.20210819153322-82a2abf51252
	k8s.io/api v0.22.0
	k8s.io/apimachinery v0.22.0
	k8s.io/cli-runtime v0.22.0
	k8s.io/client-go v0.22.0
	k8s.io/cloud-provider v0.22.0
	k8s.io/component-base v0.22.0
	k8s.io/cri-api v0.22.0
	k8s.io/kube-aggregator v0.22.0
	k8s.io/kubectl v0.22.0
	k8s.io/mount-utils v0.22.0
	k8s.io/system-validators v1.4.0
	k8s.io/utils v0.0.0-20210707171843-4b05e18ac7d9
	rsc.io/letsencrypt v0.0.3 // indirect
)

replace (
	github.com/Microsoft/go-winio => github.com/Microsoft/go-winio v0.4.19
	github.com/docker/libnetwork => github.com/moby/libnetwork v0.8.0-dev.2.0.20201031180254-535ef365dc1d
	github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
)

// backported from k8s upstream, as a project which uses etcd, containerd and grpc at the same time, they have already selected versions which don't provide any interface compile time errors
replace (
	github.com/containerd/continuity => github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc
	github.com/containerd/ttrpc => github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl => github.com/containerd/typeurl v1.0.1
	github.com/docker/distribution => github.com/docker/distribution v2.7.1+incompatible
	github.com/docker/docker => github.com/docker/docker v20.10.2+incompatible
)
