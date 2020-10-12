package applier

import (
	"bytes"
	"context"
	"io/ioutil"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/retry"
	"path"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"gopkg.in/fsnotify.v1"

	"github.com/Mirantis/mke/pkg/constant"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
)

// Applier manages all the "static" manifests and applies them on the k8s API
type Applier struct {
	Name string
	Dir  string

	log             *logrus.Entry
	watcher         *fsnotify.Watcher
	client          dynamic.Interface
	discoveryClient discovery.CachedDiscoveryInterface
}

// NewApplier creates new Applier
func NewApplier(dir string) Applier {
	name := filepath.Base(dir)
	log := logrus.WithFields(logrus.Fields{
		"component": "applier",
		"bundle":    name,
	})

	return Applier{
		log:  log,
		Dir:  dir,
		Name: name,
	}
}

func (a *Applier) init() error {
	cfg, err := clientcmd.BuildConfigFromFlags("", constant.AdminKubeconfigConfigPath)
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

// Apply resources
func (a *Applier) Apply() error {
	if a.client == nil {
		err := retry.OnError(retry.DefaultBackoff, func(err error) bool {
			return true
		}, a.init)

		if err != nil {
			return err
		}
	}
	files, err := filepath.Glob(path.Join(a.Dir, "*.yaml"))
	if err != nil {
		return err
	}
	resources, err := a.parseFiles(files)
	stack := Stack{
		Name:      a.Name,
		Resources: resources,
		Client:    a.client,
		Discovery: a.discoveryClient,
	}
	a.log.Debug("applying stack")
	err = stack.Apply(context.Background(), true)
	if err != nil {
		a.log.WithError(err).Warn("stack apply failed")
		a.discoveryClient.Invalidate()
	} else {
		a.log.Debug("successfully applied stack")
	}

	return err
}

// Delete deletes the entire stack by applying it with empty set of resources
func (a *Applier) Delete() error {
	stack := Stack{
		Name:      a.Name,
		Resources: []*unstructured.Unstructured{},
		Client:    a.client,
		Discovery: a.discoveryClient,
	}
	logrus.Debugf("about to delete a stack %s with empty apply", a.Name)
	err := stack.Apply(context.Background(), true)
	return err
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
			if item.GetAPIVersion() != "" && item.GetKind() != "" {
				resources = append(resources, item)
				resource = nil
			}
		}
	}

	return resources, nil
}
