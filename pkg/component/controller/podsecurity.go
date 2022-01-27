/*
Copyright 2022 k0s authors

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
package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-openapi/jsonpointer"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	v1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/k0sproject/k0s/pkg/apis/k0s.k0sproject.io/v1beta1"
	"github.com/k0sproject/k0s/pkg/component"
	"github.com/k0sproject/k0s/pkg/constant"
	k8sutil "github.com/k0sproject/k0s/pkg/kubernetes"
)

// PodSecurity implements system Pod Security Standards support using namespace labels
type PodSecurity struct {
	k0sVars       constant.CfgVars
	client        kubernetes.Interface
	nsInformer    cache.SharedIndexInformer
	clusterConfig *v1beta1.ClusterConfig
	log           *logrus.Entry
}

var _ component.Component = &PodSecurity{}
var _ component.ReconcilerComponent = &PodSecurity{}

// NewPodSecurity creates new system level RBAC reconciler
func NewPodSecurity(k0sVars constant.CfgVars, clientFactory k8sutil.ClientFactoryInterface) (*PodSecurity, error) {
	client, err := clientFactory.GetClient()
	if err != nil {
		return nil, err
	}
	return &PodSecurity{
		client:     client,
		k0sVars:    k0sVars,
		nsInformer: v1informers.NewNamespaceInformer(client, 1*time.Minute, nil),
		log:        logrus.WithField("component", "PodSecurity"),
	}, nil
}

// Init does currently nothing
func (p *PodSecurity) Init(ctx context.Context) error {
	go p.nsInformer.Run(ctx.Done())
	p.nsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				return
			}
			p.ensureNSLabels(ns, p.clusterConfig)
		},
		UpdateFunc: func(_, obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok {
				return
			}
			p.ensureNSLabels(ns, p.clusterConfig)
		},
	})
	return nil
}

// Run reconciles the k0s default PSP rules
func (p *PodSecurity) Run(_ context.Context) error {
	return nil
}

// Stop does currently nothing
func (p *PodSecurity) Stop() error {
	return nil
}

// Reconcile detects changes in configuration and applies them to the component
func (p *PodSecurity) Reconcile(ctx context.Context, clusterConfig *v1beta1.ClusterConfig) error {
	p.clusterConfig = clusterConfig
	nsList, err := p.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting namespace list: %v", err)
	}
	for _, ns := range nsList.Items {
		p.ensureNSLabels(&ns, p.clusterConfig)
	}

	return nil
}

func (p *PodSecurity) ensureNSLabels(ns *corev1.Namespace, clusterConfig *v1beta1.ClusterConfig) {
	if clusterConfig == nil {
		return
	}

	if ns.Name == "kube-system" || ns.Name == "kube-public" {
		return
	}
	if clusterConfig.Spec.PodSecurity.Enforce != "" {
		if _, ok := ns.Labels[constant.PodSecurityStandardNSLabelEnforce]; !ok {
			_, err := p.addNamespaceLabel(ns.Name, constant.PodSecurityStandardNSLabelEnforce, clusterConfig.Spec.PodSecurity.Enforce)
			if err != nil {
				p.log.Errorf("error adding namespace label %s/%s: %v", constant.PodSecurityStandardNSLabelEnforce, clusterConfig.Spec.PodSecurity.Enforce, err)
			}
		}
	}
	if clusterConfig.Spec.PodSecurity.Audit != "" {
		if _, ok := ns.Labels[constant.PodSecurityStandardNSLabelAudit]; !ok {
			_, err := p.addNamespaceLabel(ns.Name, constant.PodSecurityStandardNSLabelAudit, clusterConfig.Spec.PodSecurity.Audit)
			if err != nil {
				p.log.Errorf("error adding namespace label %s/%s: %v", constant.PodSecurityStandardNSLabelAudit, clusterConfig.Spec.PodSecurity.Audit, err)
			}
		}
	}
	if clusterConfig.Spec.PodSecurity.Warn != "" {
		if _, ok := ns.Labels[constant.PodSecurityStandardNSLabelWarn]; !ok {
			_, err := p.addNamespaceLabel(ns.Name, constant.PodSecurityStandardNSLabelWarn, clusterConfig.Spec.PodSecurity.Warn)
			if err != nil {
				p.log.Errorf("error adding namespace label %s/%s: %v", constant.PodSecurityStandardNSLabelWarn, clusterConfig.Spec.PodSecurity.Warn, err)
			}
		}
	}
}

func (p *PodSecurity) addNamespaceLabel(namespace string, key string, value string) (*corev1.Namespace, error) {
	keyPath := fmt.Sprintf("/metadata/labels/%s", jsonpointer.Escape(key))
	patch := fmt.Sprintf(`[{"op":"add", "path":"%s", "value":"%s" }]`, keyPath, value)
	return p.client.CoreV1().Namespaces().Patch(context.TODO(), namespace, types.JSONPatchType, []byte(patch), metav1.PatchOptions{})
}

// Health-check interface
func (p *PodSecurity) Healthy() error { return nil }
