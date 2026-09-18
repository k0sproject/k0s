// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"iter"
	"maps"
	"net"
	"net/netip"
	"slices"
	"strings"
	"time"

	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
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

	"github.com/asaskevich/govalidator"
	"github.com/dustin/go-humanize"
	"github.com/sirupsen/logrus"
)

const (
	// The CSRApprover won't approve any CSR whose certificate request contains
	// more SANs than this threshold. Additionally, it won't approve any CSRs
	// for nodes reporting more addresses than this threshold, since CSRs
	// created by regular kubelets have one SAN per node address. Cloud
	// providers may report every private IP of a node's network interfaces
	// as a node address, including secondary IPs allocated to pods, so the
	// threshold has to be well above the IP capacity of the largest instances.
	csrApproverMaxSANs = 4096

	// The CSRApprover won't approve any CSR whose certificate request size
	// exceeds this threshold. 512 KiB are more than enough room for 4096 IP
	// SANs plus a couple of very large DNS name SANs.
	csrApproverMaxRequestSize = 512 * 1024
)

type CSRApprover struct {
	log  logrus.FieldLogger
	stop context.CancelFunc

	ClusterConfig     *v1beta1.ClusterConfig
	KubeClientFactory kubeutil.ClientFactoryInterface
	leaderElector     leaderelector.Interface
	clientset         clientset.Interface
	nodeIdentifier    nodeidentifier.NodeIdentifier
	serviceCIDRs      []netip.Prefix
	clusterDomain     string
}

var _ manager.Component = (*CSRApprover)(nil)

// NewCSRApprover creates the CSRApprover component
func NewCSRApprover(c *v1beta1.ClusterConfig, leaderElector leaderelector.Interface, kubeClientFactory kubeutil.ClientFactoryInterface, network *v1beta1.Network) *CSRApprover {
	cidrs := []string{network.ServiceCIDR}
	if network.DualStack.Enabled {
		cidrs = append(cidrs, network.DualStack.IPv6ServiceCIDR)
	}
	prefixes := make([]netip.Prefix, len(cidrs))
	for i := range cidrs {
		// Need to use net.ParseCIDR here as this is what the validation uses,
		// and it's more permissive than netip.ParsePrefix, i.e. it allows
		// leading zeroes for the bit count.
		_, ipNet, err := net.ParseCIDR(cidrs[i])
		if err != nil {
			panic(err) // CIDR validity is checked early on during config validation
		}

		// Below calls can't fail for net.IPNet values produced by net.ParseCIDR.
		addr, _ := netip.AddrFromSlice(ipNet.IP)
		prefixLen, _ := ipNet.Mask.Size()

		// Construct a normalized prefix.
		prefixes[i] = k0snet.UnmapPrefix(netip.PrefixFrom(addr, prefixLen)).Masked()
	}

	return &CSRApprover{
		ClusterConfig:     c,
		leaderElector:     leaderElector,
		KubeClientFactory: kubeClientFactory,
		nodeIdentifier:    nodeidentifier.NewDefaultNodeIdentifier(),
		serviceCIDRs:      prefixes,

		// Normalize the cluster domain to be lowercase, so we can use substring
		// matches later on. This is safe, as it is proven to be ASCII only
		// during configuration validation at startup.
		clusterDomain: strings.TrimSuffix(strings.ToLower(network.ClusterDomain), "."),
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

	pending, err := a.fetchPendingCSRs(ctx)
	if err != nil {
		return nil
	}

	for csr := range pending {
		err := a.processCSR(ctx, csr)
		select {
		case <-ctx.Done():
			if err == nil {
				return nil
			}
			return fmt.Errorf("while processing %s: %w", csr.Name, err)

		default:
			if err != nil {
				a.log.WithError(err).Warnf("Failed to process CSR %q", csr.Name)
			}
		}
	}

	return nil
}

func (a *CSRApprover) fetchPendingCSRs(ctx context.Context) (iter.Seq[*certificatesv1.CertificateSigningRequest], error) {
	opts := metav1.ListOptions{
		FieldSelector: "spec.signerName=kubernetes.io/kubelet-serving",
	}

	csrs, err := a.clientset.CertificatesV1().CertificateSigningRequests().List(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("can't fetch CSRs: %w", err)
	}

	// Only consider the newest pending request of each requesting user. A
	// kubelet never has more than one request outstanding, so this doesn't hold
	// back legitimate requests, but it caps the work that a single identity can
	// cause per pass.
	pending := make(map[string]*certificatesv1.CertificateSigningRequest)
	for i := range csrs.Items {
		csr := &csrs.Items[i]
		if approved, denied := getCertApprovalCondition(&csr.Status); approved || denied {
			a.log.Debugf("CSR %s is approved=%t || denied=%t. Carry on", csr.Name, approved, denied)
			continue
		}
		if newest, ok := pending[csr.Spec.Username]; !ok || newest.CreationTimestamp.Before(&csr.CreationTimestamp) {
			pending[csr.Spec.Username] = csr
		}
	}

	// Rely on Go's randomized map iteration order, so that nobody
	// can starve others by choosing request names that sort first.
	return maps.Values(pending), nil
}

func (a *CSRApprover) processCSR(ctx context.Context, csr *certificatesv1.CertificateSigningRequest) error {
	cr, err := a.authorizeKubeletServingCSR(ctx, csr)
	if err != nil {
		if _, ok := errors.AsType[csrCheckErr](err); ok {
			return err
		}

		a.log.WithError(err).Infof("Not approving CSR %q as it is not recognized as a kubelet-serving certificate", csr.Name)
		return nil
	}

	a.log.Infof("approving csr %s with SANs: %s, IP Addresses:%s", csr.Name, cr.DNSNames, cr.IPAddresses)
	appendApprovalCondition(csr, "Auto approving kubelet serving certificate after SubjectAccessReview.")
	_, err = a.clientset.CertificatesV1().CertificateSigningRequests().UpdateApproval(ctx, csr.Name, csr, metav1.UpdateOptions{})
	return err
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
// was submitted by the very node whose identity it requests, that it doesn't
// request any names or addresses reserved for in-cluster services, that the
// requester is allowed to create such requests, and that it only requests names
// and addresses that the node reports for itself.
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

	// Never issue certificates for names and addresses that identify the
	// cluster's control plane or other in-cluster services, before even
	// checking what the node reports.
	if err := a.denyInClusterSANs(cr); err != nil {
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
	if len(spec.Request) > csrApproverMaxRequestSize {
		return nil, errors.New("certificate request size exceeds " + humanize.IBytes(csrApproverMaxRequestSize))
	}

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

	if len(cr.DNSNames)+len(cr.IPAddresses) > csrApproverMaxSANs {
		return nil, fmt.Errorf("certificate request contains more than %d SANs", csrApproverMaxSANs)
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

func (a *CSRApprover) denyInClusterSANs(cr *x509.CertificateRequest) error {
	for i, dnsName := range cr.DNSNames {
		if !govalidator.IsDNSName(dnsName) {
			return fmt.Errorf("DNSNames[%d]: invalid: %q", i, dnsName)
		}
		// Certificate verification is case-insensitive, so compare the
		// normalized form. This is safe, as dnsName is proven to be ASCII only.
		subdomain := strings.TrimSuffix(strings.ToLower(dnsName), ".")
		if subdomain == "kubernetes" || subdomain == "kubernetes.default" {
			return fmt.Errorf("DNSNames[%d]: forbidden: %q: reserved for in-cluster services", i, dnsName)
		}
		for _, domain := range [...]string{"svc", a.clusterDomain} {
			// The dnsName is a valid all-lowercase DNS name (checked above), as
			// well as a.clusterDomain (checked during configuration validation
			// at startup, and during component creation). So we can safely
			// perform a suffix match. The strings must either be equal
			// (len(prefix) == 0) or the prefix must end with a dot. A prefix
			// that consists of nothing but a dot can't occur, as a valid DNS
			// name never starts with a dot.
			prefix, ok := strings.CutSuffix(subdomain, domain)
			if !ok {
				continue
			}
			if n := len(prefix); n == 0 || prefix[n-1] == '.' {
				return fmt.Errorf("DNSNames[%d]: forbidden: %q: reserved for in-cluster services", i, dnsName)
			}
		}
	}
	for i, address := range cr.IPAddresses {
		ip, ok := netip.AddrFromSlice(address)
		if !ok {
			return fmt.Errorf("IPAddresses[%d]: invalid: %q: not an IP address", i, address)
		}
		// Unmap IPv4-mapped IPv6 addresses, as they get compared against
		// unmapped prefixes. Otherwise, they wouldn't match an IPv4 prefix.
		ip = ip.Unmap()
		for _, cidr := range a.serviceCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("IPAddresses[%d]: forbidden: %q: is inside a cluster service CIDR", i, address)
			}
		}
	}
	return nil
}

func (a *CSRApprover) verifyNode(ctx context.Context, nodeName string, cr *x509.CertificateRequest) error {
	node, err := a.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return err
		}
		return csrCheckErr{fmt.Errorf("failed to get node %q: %w", nodeName, err)}
	}
	if len(node.Status.Addresses) > csrApproverMaxSANs {
		return fmt.Errorf("node reports more than %d addresses", csrApproverMaxSANs)
	}

	// The certificate may only be valid for the names and addresses that the
	// node reports for itself. This mirrors how the kubelet builds its serving
	// certificate requests. Normalize IP addresses on both sides to mitigate
	// different but otherwise equivalent string notations.
	nodeAddresses := make(map[string]struct{})
	var errs []error
	for _, address := range node.Status.Addresses {
		address := address.Address
		// Kubernetes uses ParseIPSloppy for backwards compatibility.
		if ip := utilsnet.ParseIPSloppy(address); ip != nil {
			address = ip.String()
		}
		nodeAddresses[address] = struct{}{}
	}
	for i, dnsName := range cr.DNSNames {
		if _, ok := nodeAddresses[dnsName]; !ok {
			errs = append(errs, fmt.Errorf("DNSNames[%d]: forbidden: %q: is not a node address of %s", i, dnsName, nodeName))
		}
	}
	for i, address := range cr.IPAddresses {
		address := address.String()
		if _, ok := nodeAddresses[address]; !ok {
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
