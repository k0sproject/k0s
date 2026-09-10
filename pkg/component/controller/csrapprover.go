// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	certificates "k8s.io/kubernetes/pkg/apis/certificates"

	"github.com/sirupsen/logrus"
)

type CSRApprover struct {
	log  logrus.FieldLogger
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
	}
}

// Stop stops the CSRApprover
func (a *CSRApprover) Stop() error {
	a.stop()
	return nil
}

// Init initializes the component needs
func (a *CSRApprover) Init(ctx context.Context) error {
	a.log = k0scontext.ValueOrElse(ctx, func() logrus.FieldLogger {
		return logrus.StandardLogger()
	}).WithField("component", "csrapprover")

	var err error
	a.clientset, err = a.KubeClientFactory.GetClient()
	if err != nil {
		return fmt.Errorf("can't create kubernetes rest client for CSR check: %w", err)
	}

	return nil
}

// Run every 10 seconds checks for newly issued CSRs and approves them
func (a *CSRApprover) Start(ctx context.Context) error {
	ctx, a.stop = context.WithCancel(ctx)
	go func() {
		defer a.stop()
		ticker := time.NewTicker(10 * time.Second) // TODO: sometimes this should be refactored so it watches instead of polls for CSRs
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				err := a.approveCSR(ctx)
				if err != nil {
					a.log.WithError(err).Warn("CSR approval failed")
				}
			case <-ctx.Done():
				a.log.Info("CSR Approver context done")
				return
			}
		}
	}()

	return nil
}

type csrCheckErr struct{ error }

func (e csrCheckErr) Unwrap() error { return e.error }

// Majority of this code has been adapted from https://github.com/kontena/kubelet-rubber-stamp
func (a *CSRApprover) approveCSR(ctx context.Context) error {
	if !a.leaderElector.IsLeader() {
		a.log.Debug("not the leader, can't approve certificates")
		return nil
	}

	opts := metav1.ListOptions{
		FieldSelector: "spec.signerName=kubernetes.io/kubelet-serving",
	}

	csrs, err := a.clientset.CertificatesV1().CertificateSigningRequests().List(ctx, opts)
	if err != nil {
		return fmt.Errorf("can't fetch CSRs: %w", err)
	}

	for _, csr := range csrs.Items {
		if approved, denied := getCertApprovalCondition(&csr.Status); approved || denied {
			a.log.Debugf("CSR %s is approved=%t || denied=%t. Carry on", csr.Name, approved, denied)
			continue
		}

		cr, err := a.ensureKubeletServingCert(ctx, &csr)
		if err != nil {
			select {
			case <-ctx.Done():
				return err
			default:
			}

			log := a.log.WithError(err)
			if _, ok := errors.AsType[csrCheckErr](err); ok {
				log.Warnf("Failed to check CSR %q", csr.Name)
			} else {
				log.Infof("Not approving CSR %q as it is not recognized as a kubelet-serving certificate", csr.Name)
			}
			continue
		}

		a.log.Infof("approving csr %s with SANs: %s, IP Addresses:%s", csr.Name, cr.DNSNames, cr.IPAddresses)
		appendApprovalCondition(&csr, "Auto approving kubelet serving certificate after SubjectAccessReview.")
		_, err = a.clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, &csr, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("error updating approval for CSR %q: %w", csr.Name, err)
		}

		return nil
	}

	return nil
}

func (a *CSRApprover) authorizeCSRCreation(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) (bool, error) {
	extra := make(map[string]authorizationv1.ExtraValue)
	for k, v := range csr.Spec.Extra {
		extra[k] = authorizationv1.ExtraValue(v)
	}

	sar := &authorizationv1.SubjectAccessReview{
		Spec: authorizationv1.SubjectAccessReviewSpec{
			User:   csr.Spec.Username,
			UID:    csr.Spec.UID,
			Groups: csr.Spec.Groups,
			Extra:  extra,
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:    certificatesv1.GroupName,
				Resource: "certificatesigningrequests",
				Verb:     "create",
			},
		},
	}

	opts := metav1.CreateOptions{}
	sar, err := a.clientset.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, opts)
	if err != nil {
		return false, err
	}
	return sar.Status.Allowed, nil
}

func (a *CSRApprover) ensureKubeletServingCert(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	cr, err := validateKubeletServingCSR(&csr.Spec)
	if err != nil {
		return nil, err
	}

	if approved, err := a.authorizeCSRCreation(ctx, csr); err != nil {
		return nil, csrCheckErr{fmt.Errorf("SubjectAccessReview failed: %w", err)}
	} else if !approved {
		return nil, errors.New("requesting user is not allowed to create certificate signing requests")
	}

	return cr, nil
}

func validateKubeletServingCSR(spec *certificatesv1.CertificateSigningRequestSpec) (*x509.CertificateRequest, error) {
	cr, err := certificates.ParseCSR(spec.Request)
	if err != nil {
		return nil, fmt.Errorf("unable to parse certificate request: %w", err)
	}

	usages := sets.NewString()
	for _, usage := range spec.Usages {
		usages.Insert(string(usage))
	}

	if err := certificates.ValidateKubeletServingCSR(cr, usages); err != nil {
		return nil, err
	}

	return cr, nil
}

func getCertApprovalCondition(status *certificatesv1.CertificateSigningRequestStatus) (approved bool, denied bool) {
	for _, c := range status.Conditions {
		if c.Type == certificatesv1.CertificateApproved {
			approved = true
		}
		if c.Type == certificatesv1.CertificateDenied {
			denied = true
		}
	}
	return
}

func appendApprovalCondition(csr *certificatesv1.CertificateSigningRequest, message string) {
	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1.CertificateSigningRequestCondition{
		Type:    certificatesv1.CertificateApproved,
		Reason:  "Autoapproved by K0s CSRApprover",
		Message: message,
		Status:  corev1.ConditionTrue,
	})
}
