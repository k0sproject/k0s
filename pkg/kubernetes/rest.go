// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

type restClientGetter struct {
	clientFactory ClientFactoryInterface
	log           logrus.FieldLogger
}

func NewRESTClientGetter(clientFactory ClientFactoryInterface, log logrus.FieldLogger) resource.RESTClientGetter {
	if log == nil {
		log = logrus.NewEntry(logrus.StandardLogger())
	}
	return &restClientGetter{
		clientFactory: clientFactory,
		log:           log,
	}
}

func (r *restClientGetter) ToRESTConfig() (*rest.Config, error) {
	return r.clientFactory.GetRESTConfig(), nil
}

func (r *restClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	return r.clientFactory.GetDiscoveryClient()
}
func (r *restClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := r.clientFactory.GetDiscoveryClient()
	if err != nil {
		return nil, err
	}

	// We need to invalidate the cache. Otherwise, the client will not be aware of the new CRDs deployed after client initialization.
	discoveryClient.Invalidate()
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	expander := restmapper.NewShortcutExpander(mapper, discoveryClient, func(warning string) {
		r.log.Warn("Shortcut expansion: ", warning)
	})

	return expander, nil
}
