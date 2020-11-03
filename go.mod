module github.com/k0sproject/k0s

go 1.13

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/bmizerany/assert v0.0.0-20160611221934-b7ed37b82869 // indirect
	github.com/cloudflare/cfssl v1.4.1
	github.com/coreos/go-semver v0.3.0 // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e // indirect
	github.com/coreos/pkg v0.0.0-20180928190104-399ea9e2e55f // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-sql-driver/mysql v1.4.1 // indirect
	github.com/golangci/golangci-lint v1.31.0 // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.9
	github.com/jmoiron/sqlx v1.2.1-0.20190826204134-d7d95172beb5 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lib/pq v1.8.0 // indirect
	github.com/magiconair/properties v1.8.1
	github.com/mattn/go-sqlite3 v1.14.4 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/segmentio/backo-go v0.0.0-20200129164019-23eae7c10bd3 // indirect
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/tmc/grpc-websocket-proxy v0.0.0-20190109142713-0ad062ec5ee5 // indirect
	github.com/urfave/cli/v2 v2.2.0
	github.com/weaveworks/footloose v0.0.0-20200609124411-8f3df89ea188
	github.com/xtgo/uuid v0.0.0-20140804021211-a0b114877d4c // indirect
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	go.uber.org/atomic v1.4.0 // indirect
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76
	golang.org/x/sys v0.0.0-20200905004654-be1d3432aa8f // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/segmentio/analytics-go.v3 v3.1.0
	gopkg.in/yaml.v2 v2.3.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	k8s.io/api v0.19.1
	k8s.io/apimachinery v0.19.1
	k8s.io/client-go v0.19.1
	sigs.k8s.io/controller-runtime v0.6.3
)

// We need to force to a git commit of 3.4.13 release, see https://github.com/etcd-io/etcd/issues/12109
replace go.etcd.io/etcd => github.com/etcd-io/etcd v0.5.0-alpha.5.0.20200824191128-ae9734ed278b
