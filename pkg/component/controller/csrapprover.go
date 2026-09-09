// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/k0scontext"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"

	authorizationv1 "k8s.io/api/authorization/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	clientset "k8s.io/client-go/kubernetes"
	certificates "k8s.io/kubernetes/pkg/apis/certificates"
	"k8s.io/kubernetes/pkg/auth/nodeidentifier"
	utilsnet "k8s.io/utils/net"

	"github.com/sirupsen/logrus"
)

type CSRApprover struct {
	log  logrus.FieldLogger
	stop context.CancelFunc

	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface
	leaderElector     leaderelector.Interface
	clientset         clientset.Interface
	nodeIdentifier    nodeidentifier.NodeIdentifier
}

var _ manager.Component = (*CSRApprover)(nil)

// NewCSRApprover creates the CSRApprover component
func NewCSRApprover(c *v1beta1.ClusterConfig, leaderElector leaderelector.Interface, kubeClientFactory kubeutil.ClientFactoryInterface) *CSRApprover {
	return &CSRApprover{
		ClusterConfig:     c,
		leaderElector:     leaderElector,
		KubeClientFactory: kubeClientFactory,
		nodeIdentifier:    nodeidentifier.NewDefaultNodeIdentifier(),
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

// Checks roughly every 10 seconds for newly issued kubelet-serving CSRs and
// approves them if they meet all the criteria.
func (a *CSRApprover) Start(ctx context.Context) error {
	ctx, a.stop = context.WithCancel(ctx)
	go func() {
		defer a.stop()

		// TODO: sometimes this should be refactored so it watches instead of polls for CSRs
		wait.JitterUntilWithContext(ctx, func(ctx context.Context) {
			err := a.approveCSR(ctx)
			if err != nil {
				a.log.WithError(err).Warn("CSR approval failed")
			}
		}, 8*time.Second, 0.5, true)

		a.log.Info("CSR Approver context done")
	}()

	return nil
}

type csrCheckErr struct{ error }

func (e csrCheckErr) Unwrap() error { return e.error }

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

		cr, err := a.authorizeKubeletServingCSR(ctx, &csr)
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

// Checks that the CSR is a well-formed kubelet-serving certificate request that
// was submitted by the very node whose identity it requests, that the requester
// is allowed to create such requests, and that it only requests names and
// addresses that the node reports for itself.
func (a *CSRApprover) authorizeKubeletServingCSR(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) (*x509.CertificateRequest, error) {
	// This only checks the shape of the request. Whether the requester is
	// actually entitled to impersonate the requested identity is up to k0s.
	cr, err := validateKubeletServingCSR(&csr.Spec)
	if err != nil {
		return nil, err
	}

	nodeName, err := a.verifyNodeIdentity(csr, cr)
	if err != nil {
		return nil, err
	}

	if approved, err := a.authorizeCSRCreation(ctx, csr); err != nil {
		return nil, csrCheckErr{fmt.Errorf("SubjectAccessReview failed: %w", err)}
	} else if !approved {
		return nil, errors.New("requesting user is not allowed to create certificate signing requests")
	}

	if err := a.verifyNode(ctx, nodeName, cr); err != nil {
		return nil, err
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

func (a *CSRApprover) verifyNodeIdentity(csr *certificatesv1.CertificateSigningRequest, cr *x509.CertificateRequest) (string, error) {
	requestingUser := (*requestingUserInfo)(&csr.Spec)
	nodeName, isNode := a.nodeIdentifier.NodeIdentity(requestingUser)
	if !isNode {
		return "", fmt.Errorf("not requested by a node identity: user %q in group(s) %q", requestingUser.GetName(), requestingUser.GetGroups())
	}
	if errs := validation.IsDNS1123Subdomain(nodeName); len(errs) > 0 {
		return "", fmt.Errorf("requesting node name %q is invalid: %s", nodeName, strings.Join(errs, "; "))
	}

	// The CSR must have been created by the user it requests a certificate for.
	if requested, requesting := cr.Subject.CommonName, csr.Spec.Username; requested != requesting {
		return "", fmt.Errorf("requested subject's common name %q doesn't match requesting user name %q", requested, requesting)
	}
	return nodeName, nil
}

func (a *CSRApprover) verifyNode(ctx context.Context, nodeName string, cr *x509.CertificateRequest) error {
	node, err := a.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}
		return csrCheckErr{fmt.Errorf("failed to get node %q: %w", nodeName, err)}
	}

	// The certificate may only be valid for the names and addresses that the
	// node reports for itself. This mirrors how the kubelet builds its serving
	// certificate requests. Normalize IP addresses on both sides to mitigate
	// different but otherwise equivalent string notations.
	var (
		nodeAddresses []string
		errs          []error
	)
	for _, address := range node.Status.Addresses {
		address := address.Address
		// Kubernetes uses ParseIPSloppy for backwards compatibility.
		if ip := utilsnet.ParseIPSloppy(address); ip != nil {
			address = ip.String()
		}
		if !slices.Contains(nodeAddresses, address) {
			nodeAddresses = append(nodeAddresses, address)
		}
	}
	for i, dnsName := range cr.DNSNames {
		if !slices.Contains(nodeAddresses, dnsName) {
			errs = append(errs, fmt.Errorf("DNSNames[%d]: forbidden: %q: is not a node address of %s", i, dnsName, nodeName))
		}
	}
	for i, address := range cr.IPAddresses {
		if address := address.String(); !slices.Contains(nodeAddresses, address) {
			errs = append(errs, fmt.Errorf("IPAddresses[%d]: forbidden: %q: is not a node address of %s", i, address, nodeName))
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
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

type requestingUserInfo certificatesv1.CertificateSigningRequestSpec

var _ user.Info = (*requestingUserInfo)(nil) // Also pins the user package import for godoc.

func (i *requestingUserInfo) GetName() string     { return i.Username } // GetName implements [user.Info].
func (i *requestingUserInfo) GetUID() string      { return i.UID }      // GetUID implements [user.Info].
func (i *requestingUserInfo) GetGroups() []string { return i.Groups }   // GetGroups implements [user.Info].

// GetExtra implements [user.Info].
func (i *requestingUserInfo) GetExtra() map[string][]string {
	// Need a deep clone to adapt the types ¯\_(ツ)_/¯
	extra := make(map[string][]string, len(i.Extra))
	for k, v := range i.Extra {
		extra[k] = slices.Clone(v)
	}
	return extra
}
