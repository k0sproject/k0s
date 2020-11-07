/*
Copyright 2020 Mirantis, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clientset

import (
	"github.com/Mirantis/mke/pkg/apis/helm.k0sproject.io/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"context"
)

const resourceName = "charts"

// ChartV1Beta1Interface typed client interface
type ChartV1Beta1Interface interface {
	Charts(namespace string) ChartInterface
}

// ChartV1Beta1Client typed client instance
type ChartV1Beta1Client struct {
	restClient rest.Interface
}

// Charts returns charts typed client for given namespace
func (c ChartV1Beta1Client) Charts(namespace string) ChartInterface {
	return &chartClient{
		ns:         namespace,
		restClient: c.restClient,
	}
}

// ChartInterface typed client methods set
type ChartInterface interface {
	Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)
	List(ctx context.Context) (*v1beta1.ChartList, error)
	Create(ctx context.Context, chart *v1beta1.Chart) (*v1beta1.Chart, error)
}

type chartClient struct {
	restClient rest.Interface
	ns         string
}

// Watch watches for changes in charts
func (c chartClient) Watch(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.restClient.
		Get().
		Namespace(c.ns).
		Resource(resourceName).
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch(ctx)
}

// List lists charts
func (c chartClient) List(ctx context.Context) (*v1beta1.ChartList, error) {
	result := v1beta1.ChartList{}

	err := c.restClient.
		Get().
		Namespace(c.ns).
		Resource(resourceName).
		Do(ctx).
		Into(&result)

	return &result, err
}

// Create creates chart
func (c chartClient) Create(ctx context.Context, chart *v1beta1.Chart) (*v1beta1.Chart, error) {
	resBody := &v1beta1.Chart{}
	err := c.restClient.
		Post().
		Namespace(c.ns).
		Resource(resourceName).
		Body(chart).
		Do(ctx).
		Into(resBody)
	return resBody, err
}

// NewForConfig build new chart client
func NewForConfig(cfgPath string) (*ChartV1Beta1Client, error) {
	config, err := clientcmd.BuildConfigFromFlags("", cfgPath)
	if err != nil {
		return nil, err
	}
	if err := v1beta1.AddToScheme(scheme.Scheme); err != nil {
		return nil, err
	}
	crdConfig := *config
	crdConfig.GroupVersion = &v1beta1.GroupVersion
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()
	restClient, err := rest.RESTClientFor(&crdConfig)
	if err != nil {
		return nil, err
	}
	return &ChartV1Beta1Client{restClient: restClient}, nil
}
