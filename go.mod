module github.com/k0sproject/k0s

go 1.13

require (
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/Microsoft/hcsshim v0.8.7
	github.com/asaskevich/govalidator v0.0.0-20200428143746-21a406dcc535
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cloudflare/cfssl v1.4.1
	github.com/containerd/containerd v1.4.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/docker/libnetwork v0.5.6
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/go-openapi/jsonpointer v0.19.3
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.8
	github.com/ishidawataru/sctp v0.0.0-20191218070446-00ab2ac2db07 // indirect
	github.com/jmoiron/sqlx v1.2.1-0.20190826204134-d7d95172beb5 // indirect
	github.com/k0sproject/dig v0.1.0
	github.com/kardianos/service v1.2.1-0.20210616011951-36c9bf8c36a2
	github.com/kr/text v0.2.0 // indirect
	github.com/magiconair/properties v1.8.1
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mitchellh/go-ps v1.0.0
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/olekukonko/tablewriter v0.0.2
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/opencontainers/selinux v1.8.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/rogpeppe/go-internal v1.6.1 // indirect
	github.com/rqlite/rqlite v0.0.0-20210528155034-8dc8788f37db
	github.com/segmentio/analytics-go v3.1.0+incompatible
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	github.com/urfave/cli v1.22.2
	github.com/vishvananda/netlink v1.1.0 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/weaveworks/footloose v0.0.0-20200609124411-8f3df89ea188
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	github.com/zcalusic/sysinfo v0.0.0-20210226105846-b810d137e525
	go.etcd.io/etcd v0.5.0-alpha.5.0.20200910180754-dd1b699fc489
	go.uber.org/zap v1.13.0
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76
	golang.org/x/tools v0.0.0-20201013201025-64a9e34f3752 // indirect
	google.golang.org/grpc v1.27.1
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	helm.sh/helm/v3 v3.4.0
	honnef.co/go/tools v0.0.1-2020.1.6 // indirect
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/cli-runtime v0.20.2
	k8s.io/client-go v0.20.5
	k8s.io/cloud-provider v0.20.5
	k8s.io/cri-api v0.20.4
	k8s.io/kube-aggregator v0.20.5
	k8s.io/kubectl v0.20.2
	k8s.io/mount-utils v0.20.4
	k8s.io/system-validators v1.4.0
	k8s.io/utils v0.0.0-20201110183641-67b214c5f920
)

// We need to force to a git commit of 3.4.13 release, see https://github.com/etcd-io/etcd/issues/12109
replace go.etcd.io/etcd => github.com/etcd-io/etcd v0.5.0-alpha.5.0.20200824191128-ae9734ed278b

replace (
	github.com/docker/distribution => github.com/docker/distribution v0.0.0-20191216044856-a8371794149d
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/docker/libnetwork => github.com/moby/libnetwork v0.8.0-dev.2.0.20201031180254-535ef365dc1d
)

// backported from k8s upstream, as a project which uses etcd, containerd and grpc at the same time, they have already selected versions which don't provide any interface compile time errors
replace (
	github.com/containerd/cgroups => github.com/containerd/cgroups v0.0.0-20200531161412-0dbf7f05ba59
	github.com/containerd/console => github.com/containerd/console v1.0.0
	github.com/containerd/containerd => github.com/containerd/containerd v1.4.1
	github.com/containerd/continuity => github.com/containerd/continuity v0.0.0-20190426062206-aaeac12a7ffc
	github.com/containerd/fifo => github.com/containerd/fifo v0.0.0-20190226154929-a9fb20d87448
	github.com/containerd/go-runc => github.com/containerd/go-runc v0.0.0-20180907222934-5a6d9f37cfa3
	github.com/containerd/ttrpc => github.com/containerd/ttrpc v1.0.2
	github.com/containerd/typeurl => github.com/containerd/typeurl v1.0.1
	google.golang.org/grpc => google.golang.org/grpc v1.27.1
)

replace github.com/kardianos/service => github.com/k0sproject/kardianos-service v1.2.1-0.20210701120136-4fa800c407da
