// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	authorization "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	certificates "k8s.io/kubernetes/pkg/apis/certificates"
)

type CSRApprover struct {
	log  *logrus.Entry
	stop context.CancelFunc

	KubeClientFactory kubeutil.ClientFactoryInterface
	leaderElector     leaderelector.Interface
	clientset         clientset.Interface
}

var _ manager.Component = (*CSRApprover)(nil)

// NewCSRApprover creates the CSRApprover component
func NewCSRApprover(leaderElector leaderelector.Interface, kubeClientFactory kubeutil.ClientFactoryInterface) *CSRApprover {
	return &CSRApprover{
		leaderElector:     leaderElector,
		KubeClientFactory: kubeClientFactory,
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
		return fmt.Errorf("can't create kubernetes rest client for CSR check: %w", err)
	}

	return nil
}

// kubeletServingSignerSelector limits the watch to CSRs requesting a
// kubelet-serving certificate, the only kind this approver ever acts on.
var kubeletServingSignerSelector = fields.OneTermEqualSelector("spec.signerName", "kubernetes.io/kubelet-serving")

// Start watches for CSRs requesting a kubelet-serving certificate and
// approves them as they show up, instead of polling the API on a fixed
// interval.
func (a *CSRApprover) Start(ctx context.Context) error {
	ctx, a.stop = context.WithCancel(ctx)
	go a.watchCSRs(ctx)

	return nil
}

// watchCSRs runs until ctx is done, reacting to every CSR that matches
// kubeletServingSignerSelector, be it already present or newly created.
func (a *CSRApprover) watchCSRs(ctx context.Context) {
	defer a.stop()

	var lastObservedVersion string
	err := watch.CertificateSigningRequests(a.clientset.CertificatesV1().CertificateSigningRequests()).
		WithFieldSelector(kubeletServingSignerSelector).
		WithErrorCallback(func(err error) (time.Duration, error) {
			retryDelay, e := watch.IsRetryable(err)
			if e == nil {
				a.log.WithError(err).Debugf(
					"Encountered transient error while watching CSRs"+
						", last observed resource version was %q"+
						", retrying in %s",
					lastObservedVersion, retryDelay,
				)
				return retryDelay, nil
			}
			a.log.WithError(e).Error("bailing out CSR watch")
			return 0, err
		}).
		Until(ctx, func(csr *v1.CertificateSigningRequest) (bool, error) {
			lastObservedVersion = csr.ResourceVersion
			if err := a.approveCSR(ctx, csr); err != nil {
				a.log.WithError(err).Warn("CSR approval failed")
			}
			return false, nil // never stop watching
		})

	if canceled := context.Cause(ctx); errors.Is(err, canceled) {
		a.log.WithError(err).Info("CSR watch terminated")
	} else {
		a.log.WithError(err).Error("CSR watch terminated unexpectedly")
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
		return fmt.Errorf("unable to parse csr %q: %w", csr.Name, err)
	}

	if err := a.ensureKubeletServingCert(csr, x509cr); err != nil {
		a.log.WithError(err).Infof("Not approving CSR %q as it is not recognized as a kubelet-serving certificate", csr.Name)
		return nil
	}

	approved, err := a.authorize(ctx, csr, authorization.ResourceAttributes{
		Group:    "certificates.k8s.io",
		Resource: "certificatesigningrequests",
		Verb:     "create",
	})
	if err != nil {
		return fmt.Errorf("SubjectAccessReview failed for CSR %q: %w", csr.Name, err)
	}

	if !approved {
		return fmt.Errorf("failed to perform SubjectAccessReview for CSR %q", csr.Name)
	}

	a.log.Infof("approving csr %s with SANs: %s, IP Addresses:%s", csr.Name, x509cr.DNSNames, x509cr.IPAddresses)
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
		return nil, errors.New("PEM block type must be CERTIFICATE REQUEST")
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
