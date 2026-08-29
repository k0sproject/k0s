// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	certv1 "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestBasicCRSApprover(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	client, err := fakeFactory.GetClient()
	assert.NoError(t, err)

	ctx := t.Context()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	req := pemWithPrivateKey(privateKey)

	csrReq := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csrapprover_test",
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Request:    req,
			SignerName: "kubernetes.io/kubelet-serving",
		},
	}

	newCsr, err := client.CertificatesV1().CertificateSigningRequests().Create(ctx, csrReq, metav1.CreateOptions{})
	assert.NoError(t, err)

	c := NewCSRApprover(leaderelector.Off(), fakeFactory)

	assert.NoError(t, c.Init(ctx))
	assert.NoError(t, c.approveCSR(ctx, newCsr))

	csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, newCsr.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, csr)
	assert.Equal(t, newCsr.Name, csr.Name)
	for _, c := range csr.Status.Conditions {
		assert.True(t, c.Type == certv1.CertificateApproved && c.Reason == "Autoapproved by K0s CSRApprover" && c.Status == core.ConditionTrue)
	}
}

// TestCSRApproverWatch verifies that the approver reacts to a CSR as soon as
// it's created, without waiting for a poll interval: the trigger mechanism is
// a watch rather than a timer.
func TestCSRApproverWatch(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	client, err := fakeFactory.GetClient()
	require.NoError(t, err)

	// The fake clientset doesn't implement SubjectAccessReview, so make it
	// always allow, mirroring an RBAC setup that permits the approver to
	// approve kubelet-serving CSRs.
	fakeClientset, ok := client.(*kubernetesfake.Clientset)
	require.True(t, ok, "expected a fake clientset")
	fakeClientset.PrependReactor("create", "subjectaccessreviews", func(clientgotesting.Action) (bool, runtime.Object, error) {
		return true, &authorizationv1.SubjectAccessReview{
			Status: authorizationv1.SubjectAccessReviewStatus{Allowed: true},
		}, nil
	})

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	c := NewCSRApprover(leaderelector.Off(), fakeFactory)
	require.NoError(t, c.Init(ctx))
	require.NoError(t, c.Start(ctx))
	t.Cleanup(func() { assert.NoError(t, c.Stop()) })

	csrReq := &certv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: "csrapprover_watch_test",
		},
		Spec: certv1.CertificateSigningRequestSpec{
			Request:    pemWithValidKubeletServingTemplate(privateKey),
			SignerName: "kubernetes.io/kubelet-serving",
			Usages: []certv1.KeyUsage{
				certv1.UsageDigitalSignature,
				certv1.UsageKeyEncipherment,
				certv1.UsageServerAuth,
			},
		},
	}

	newCsr, err := client.CertificatesV1().CertificateSigningRequests().Create(ctx, csrReq, metav1.CreateOptions{})
	require.NoError(t, err)

	// This would only succeed within the test's timeout if the approver
	// reacts to the CSR being created, rather than waiting for its next poll
	// tick (which no longer exists).
	require.NoError(t, watch.CertificateSigningRequests(client.CertificatesV1().CertificateSigningRequests()).
		WithObjectName(newCsr.Name).
		Until(ctx, func(csr *certv1.CertificateSigningRequest) (bool, error) {
			approved, _ := getCertApprovalCondition(&csr.Status)
			return approved, nil
		}),
	)
}

// pemWithValidKubeletServingTemplate builds a CSR that satisfies
// certificates.ValidateKubeletServingCSR, so that the approver actually
// recognizes and approves it.
func pemWithValidKubeletServingTemplate(pk crypto.PrivateKey) []byte {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "system:node:test-node",
			Organization: []string{"system:nodes"},
		},
		DNSNames:    []string{"test-node"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	return pemWithTemplate(template, pk)
}

func pemWithPrivateKey(pk crypto.PrivateKey) []byte {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "something",
			Organization: []string{"test"},
		},
	}
	return pemWithTemplate(template, pk)
}

func pemWithTemplate(template *x509.CertificateRequest, key crypto.PrivateKey) []byte {
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, template, key)
	if err != nil {
		panic(err)
	}

	csrPemBlock := &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csrDER,
	}

	p := pem.EncodeToMemory(csrPemBlock)
	if p == nil {
		panic("invalid pem block")
	}

	return p
}
