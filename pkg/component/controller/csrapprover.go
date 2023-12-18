/*
Copyright 2021 k0s authors

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
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	authorization "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"k8s.io/kubernetes/pkg/apis/certificates"
)

type CSRApprover struct {
	log  *logrus.Entry
	stop context.CancelFunc

	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface
	leaderElector     leaderelector.Interface
	clientset         clientset.Interface
	resyncPeriod      time.Duration
}

var _ manager.Component = (*CSRApprover)(nil)

// NewCSRApprover creates the CSRApprover component
func NewCSRApprover(c *v1beta1.ClusterConfig, leaderElector leaderelector.Interface, kubeClientFactory kubeutil.ClientFactoryInterface, cacheResyncPeriod time.Duration) *CSRApprover {
	return &CSRApprover{
		ClusterConfig:     c,
		leaderElector:     leaderElector,
		KubeClientFactory: kubeClientFactory,
		resyncPeriod:      cacheResyncPeriod,
		log:               logrus.WithFields(logrus.Fields{"component": "csrapprover"}),
	}
}

// Stop stops the CSRApprover
func (a *CSRApprover) Stop() error {
	a.stop()
	return nil
}

// Init initializes the component needs
func (a *CSRApprover) Init(_ context.Context) error {
	var err error
	a.clientset, err = a.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for CSR check: %v", err)
	}

	return nil
}

// Start watches for newly issued CSRs and approves them.
func (a *CSRApprover) Start(ctx context.Context) error {
	ctx, a.stop = context.WithCancel(ctx)

	// TODO: share informer factory with other components.
	factory := informers.NewSharedInformerFactoryWithOptions(a.clientset, a.resyncPeriod, informers.WithTweakListOptions(func(options *metav1.ListOptions) {
		options.FieldSelector = "spec.signerName=kubernetes.io/kubelet-serving"
	}))
	_, err := factory.Certificates().V1().CertificateSigningRequests().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			a.retryApproveCSR(ctx, obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			a.retryApproveCSR(ctx, newObj)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to add event handler to shared informer: %w", err)
	}

	factory.Start(ctx.Done())
	synced := factory.WaitForCacheSync(ctx.Done())
	for _, ok := range synced {
		if !ok {
			return errors.New("caches failed to sync")
		}
	}
	return nil
}

func (a *CSRApprover) retryApproveCSR(ctx context.Context, obj interface{}) {
	csr, ok := obj.(*v1.CertificateSigningRequest)
	if !ok {
		a.log.Errorf("expected resource to be of type %T; got %T", &v1.CertificateSigningRequest{}, obj)
		return
	}

	const maxAttempts = 10
	logger := a.log.WithField("csrName", csr.Name)

	err := retry.Do(func() error {
		if err := a.approveCSR(ctx, csr); err != nil {
			logger.WithError(err).Warn("CSR approval failed")
			return err
		}
		return nil
	},
		retry.Context(ctx),
		retry.Attempts(maxAttempts),
		retry.OnRetry(func(attempts uint, err error) {
			logger.WithField("attempts", attempts).WithError(err).Info("retrying CSR approval")
		}),
	)

	if err != nil {
		logger.WithError(err).Errorf("Failed to approve CSR after %d attempts", maxAttempts)
	} else {
		logger.Info("CSR approved successfully")
	}
}

// Majority of this code has been adapted from https://github.com/kontena/kubelet-rubber-stamp
func (a *CSRApprover) approveCSR(ctx context.Context, csr *v1.CertificateSigningRequest) error {
	if !a.leaderElector.IsLeader() {
		a.log.Debug("not the leader, can't approve certificates")
		return nil
	}

	if approved, denied := getCertApprovalCondition(&csr.Status); approved || denied {
		a.log.Debugf("CSR %s is approved=%t || denied=%t. Carry on", csr.Name, approved, denied)
		return nil
	}

	x509cr, err := parseCSR(csr)
	if err != nil {
		return retry.Unrecoverable(fmt.Errorf("unable to parse CSR %q: %w", csr.Name, err))
	}

	if err := a.ensureKubeletServingCert(csr, x509cr); err != nil {
		a.log.WithError(err).Infof("Not approving CSR %q as it is not recognized as a kubelet-serving certificate", csr.Name)
		return nil
	}

	approved, err := a.authorize(ctx, csr, authorization.ResourceAttributes{
		Group:    v1.GroupName,
		Resource: "certificatesigningrequests",
		Verb:     "create",
	})
	if err != nil {
		return fmt.Errorf("SubjectAccessReview failed for CSR %q: %w", csr.Name, err)
	}

	if !approved {
		return fmt.Errorf("failed to perform SubjectAccessReview for CSR %q", csr.Name)
	}

	a.log.Infof("approving CSR %s with SANs: %s, IP Addresses:%s", csr.ObjectMeta.Name, x509cr.DNSNames, x509cr.IPAddresses)
	appendApprovalCondition(csr, "Auto approving kubelet serving certificate after SubjectAccessReview.")
	_, err = a.clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating approval for CSR %q: %w", csr.Name, err)
	}
	return nil
}

func (a *CSRApprover) authorize(ctx context.Context, csr *v1.CertificateSigningRequest, rattrs authorization.ResourceAttributes) (bool, error) {
	extra := make(map[string]authorization.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorization.ExtraValue(v)
	}

	sar := &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			User:               csr.Spec.Username,
			UID:                csr.Spec.UID,
			Groups:             csr.Spec.Groups,
			Extra:              extra,
			ResourceAttributes: &rattrs,
		},
	}

	opts := metav1.CreateOptions{}
	sar, err := a.clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, opts)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func (a *CSRApprover) ensureKubeletServingCert(csr *v1.CertificateSigningRequest, x509cr *x509.CertificateRequest) error {
	usages := sets.NewString()
	for _, usage := range csr.Spec.Usages {
		usages.Insert(string(usage))
	}

	return certificates.ValidateKubeletServingCSR(x509cr, usages)
}

func getCertApprovalCondition(status *v1.CertificateSigningRequestStatus) (approved bool, denied bool) {
	for _, c := range status.Conditions {
		if c.Type == v1.CertificateApproved {
			approved = true
		}
		if c.Type == v1.CertificateDenied {
			denied = true
		}
	}
	return
}

// parseCSR extracts the CSR from the API object and decodes it.
func parseCSR(obj *v1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	// extract PEM from request object
	pemBytes := obj.Spec.Request
	block, _ := pem.Decode(pemBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("PEM block type must be CERTIFICATE REQUEST")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return nil, err
	}
	return csr, nil
}

func appendApprovalCondition(csr *v1.CertificateSigningRequest, message string) {
	csr.Status.Conditions = append(csr.Status.Conditions, v1.CertificateSigningRequestCondition{
		Type:    v1.CertificateApproved,
		Reason:  "Autoapproved by K0s CSRApprover",
		Message: message,
		Status:  core.ConditionTrue,
	})
}
