module github.com/Mirantis/mke

go 1.13

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/avast/retry-go v2.6.0+incompatible
	github.com/cloudflare/cfssl v1.4.1
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/davecgh/go-spew v1.1.1
	github.com/denisbrodbeck/machineid v1.0.1
	github.com/evanphx/json-patch v4.9.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible // indirect
	github.com/golangci/golangci-lint v1.31.0 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/huandu/xstrings v1.3.2 // indirect
	github.com/imdario/mergo v0.3.6
	github.com/magiconair/properties v1.8.1
	github.com/mikefarah/yq/v3 v3.0.0-20200930232032-e0f5cb3c5958 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.6.0
	github.com/stretchr/testify v1.6.1
	github.com/urfave/cli/v2 v2.2.0
	github.com/weaveworks/footloose v0.0.0-20200609124411-8f3df89ea188
	go.etcd.io/etcd v0.0.0-20191023171146-3cf2f69b5738
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/sync v0.0.0-20200930132711-30421366ff76 // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.3.0
	k8s.io/api v0.19.1
	k8s.io/apimachinery v0.19.1
	k8s.io/client-go v0.19.1
)

// We need to force to a git commit of 3.4.13 release, see https://github.com/etcd-io/etcd/issues/12109
replace go.etcd.io/etcd => github.com/etcd-io/etcd v0.5.0-alpha.5.0.20200824191128-ae9734ed278b
