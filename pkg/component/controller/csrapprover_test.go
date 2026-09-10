// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"cmp"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/k0sproject/k0s/pkg/k0scontext"

	authorizationv1 "k8s.io/api/authorization/v1"
	certv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestCSRApprover(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "csr-approver-test-node"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Type: corev1.NodeHostName, Address: "csr-approver-test-node"},
				{Type: corev1.NodeInternalIP, Address: "10.0.0.1"},
			},
		},
	}

	validTemplate := func() *x509.CertificateRequest {
		return &x509.CertificateRequest{
			Subject: pkix.Name{
				CommonName:   "system:node:csr-approver-test-node",
				Organization: []string{"system:nodes"},
			},
			DNSNames:    []string{"csr-approver-test-node"},
			IPAddresses: []net.IP{{10, 0, 0, 1}},
		}
	}

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	type reviewMode uint8
	const (
		reviewModeAllow reviewMode = iota
		reviewModeNone
		reviewModeFail
	)

	for _, tt := range []struct {
		testCase    string
		template    *x509.CertificateRequest // gets defaulted to a valid template if nil
		reviewMode  reviewMode
		expectedLog string
		expectedErr string
	}{
		{
			testCase: "node requesting its own certificate",
		},
		{
			testCase:    "SubjectAccessReview fails",
			reviewMode:  reviewModeFail,
			expectedLog: "Failed to check CSR",
			expectedErr: "SubjectAccessReview failed",
		},
		{
			testCase:    "SubjectAccessReview not allowed",
			reviewMode:  reviewModeNone,
			expectedErr: "requesting user is not allowed to create certificate signing requests",
		},
		{
			testCase: "certificate that is not a kubelet-serving certificate",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.Subject.Organization = nil
				return t
			}(),
			expectedErr: "subject organization is not system:nodes",
		},
	} {
		t.Run(tt.testCase, func(t *testing.T) {
			if tt.template == nil {
				tt.template = validTemplate()
			}

			synctest.Test(t, func(t *testing.T) {
				fakeFactory := testutil.NewFakeClientFactory(node)
				client := fakeFactory.Client.(*kubernetesfake.Clientset)

				if tt.reviewMode != reviewModeNone {
					// Allow the SubjectAccessReview, so that it's actually the
					// approver's own checks that decide about the approval.
					client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
						if tt.reviewMode == reviewModeFail {
							return true, nil, assert.AnError
						}
						sar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
						sar.Status.Allowed = true
						return true, sar, nil
					})
				}

				csr := &certv1.CertificateSigningRequest{
					ObjectMeta: metav1.ObjectMeta{Name: "csrapprover_test"},
					Spec: certv1.CertificateSigningRequestSpec{
						Request:    pemWithTemplate(tt.template, privateKey),
						SignerName: certv1.KubeletServingSignerName,
						Username:   "system:node:csr-approver-test-node",
						Groups:     []string{"system:nodes"},
						Usages:     []certv1.KeyUsage{certv1.UsageDigitalSignature, certv1.UsageServerAuth},
					},
				}
				_, err := client.CertificatesV1().CertificateSigningRequests().Create(t.Context(), csr, metav1.CreateOptions{})
				require.NoError(t, err)

				logger, logs := test.NewNullLogger()
				ctx := k0scontext.WithValue[logrus.FieldLogger](t.Context(), logger)
				underTest := controller.NewCSRApprover(&v1beta1.ClusterConfig{}, &leaderelector.Dummy{Leader: true}, fakeFactory)
				require.NoError(t, underTest.Init(ctx))
				require.NoError(t, underTest.Start(ctx))
				t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

				time.Sleep(10 * time.Second)
				synctest.Wait()

				csr, err = client.CertificatesV1().CertificateSigningRequests().Get(t.Context(), csr.Name, metav1.GetOptions{})
				require.NoError(t, err)

				var approvedCond *certv1.CertificateSigningRequestCondition
				for _, c := range csr.Status.Conditions {
					if c.Type == certv1.CertificateApproved {
						approvedCond = &c
						break
					}
				}
				if tt.expectedErr != "" {
					assert.Nil(t, approvedCond, "Expected no approved condition at all")
				} else if assert.NotNil(t, approvedCond, "Expected an approved condition") {
					assert.Equalf(t, corev1.ConditionTrue, approvedCond.Status, "Unexpected status in approved condition: %v", approvedCond)
				}

				entries := logs.AllEntries()
				require.Len(t, entries, 1, "Expected exactly one log message")
				loggedMsg, loggedErr := entries[0].Message, entries[0].Data[logrus.ErrorKey]
				if tt.expectedErr == "" {
					assert.Contains(t, loggedMsg, cmp.Or(tt.expectedLog, "approving csr csrapprover_test"))
					if loggedErr != nil {
						assert.NoErrorf(t, loggedErr.(error), "Message: %s", loggedMsg)
					}
				} else {
					assert.Contains(t, loggedMsg, cmp.Or(tt.expectedLog, "Not approving CSR"))
					if assert.NotNilf(t, loggedErr, "No error was logged: %s", loggedMsg) {
						assert.ErrorContainsf(t, loggedErr.(error), tt.expectedErr, "Message: %s", loggedMsg)
					}
				}
			})
		})
	}
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
