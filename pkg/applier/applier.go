package applier

import (
	"bytes"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/Mirantis/mke/pkg/constant"
)

type Applier struct {
	Name string
	Dir  string

	log             *logrus.Entry
	watcher         *fsnotify.Watcher
	client          dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
}

func NewApplier(dir string) (Applier, error) {
	name := filepath.Base(dir)
	log := logrus.WithFields(logrus.Fields{
		"component": "applier",
		"bundle":    name,
	})

	a := Applier{
		log:  log,
		Dir:  dir,
		Name: name,
	}

	err := a.init()
	if err != nil {
		return a, err
	}

	return a, nil
}

func (a *Applier) init() error {
	kubeConfigPath := filepath.Join(constant.CertRoot, "admin.conf")

	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return err
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}
	a.client = client

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	cachedDiscoveryClient := memory.NewMemCacheClient(discoveryClient)
	if err != nil {
		return err
	}
	a.discoveryClient = cachedDiscoveryClient

	return nil
}

func (a *Applier) Apply() error {
	files, err := filepath.Glob(path.Join(a.Dir, "*.yaml"))
	if err != nil {
		return err
	}
	resources, err := a.parseFiles(files)
	stack := Stack{
		Name:      "mke-stack",
		Resources: resources,
		Client:    a.client,
		Discovery: a.discoveryClient,
	}
	return stack.Apply(true)
}

func (a *Applier) parseFiles(files []string) ([]*unstructured.Unstructured, error) {
	resources := []*unstructured.Unstructured{}
	for _, file := range files {
		source, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, err
		}

		decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(source), 4096)
		var resource map[string]interface{}
		for decoder.Decode(&resource) == nil {
			item := &unstructured.Unstructured{
				Object: resource,
			}
			resources = append(resources, item)
			resource = nil
		}
	}

	return resources, nil
}
