module github.com/Mirantis/mke

go 1.13

require (
	github.com/cloudflare/cfssl v1.4.1
	github.com/cpuguy83/go-md2man/v2 v2.0.0 // indirect
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/gorilla/mux v1.7.4
	github.com/imdario/mergo v0.3.6 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.5.0
	github.com/stretchr/testify v1.5.1 // indirect
	github.com/urfave/cli/v2 v2.2.0
	golang.org/x/crypto v0.0.0-20200414155820-4f8f47aa7992 // indirect
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b // indirect
	golang.org/x/sys v0.0.0-20200824131525-c12d262b63d8 // indirect
	golang.org/x/text v0.3.3 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
)

replace (
	github.com/google/certificate-transparency-go => github.com/google/certificate-transparency-go v1.1.0
	go.etcd.io/etcd => go.etcd.io/etcd v3.3.24+incompatible
)
