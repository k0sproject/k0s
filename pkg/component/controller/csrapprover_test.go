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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/stretchr/testify/assert"

	authorizationv1 "k8s.io/api/authorization/v1"
	certv1 "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBasicCRSApprover(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	req := pemWithPrivateKey(privateKey)

	for i, test := range []struct {
		name                             string
		startControllerBeforeCreatingCSR bool
	}{
		{
			name:                             "existing CSRs are approved",
			startControllerBeforeCreatingCSR: false,
		},
		{
			name:                             "newly-created CSRs are approved",
			startControllerBeforeCreatingCSR: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			fakeFactory := testutil.NewFakeClientFactory()
			client, err := fakeFactory.GetClient()
			assert.NoError(t, err)

			config := &v1beta1.ClusterConfig{
				Spec: &v1beta1.ClusterSpec{
					API: &v1beta1.APISpec{
						Address:         "1.2.3.4",
						ExternalAddress: "get.k0s.sh",
					},
				},
			}
			ctx := context.TODO()

			c := NewCSRApprover(config, &leaderelector.Dummy{Leader: true}, fakeFactory, 10*time.Minute)
			assert.NoError(t, c.Init(ctx))

			csrReq := &certv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("csrapprover-test-%d", i+1),
				},
				Spec: certv1.CertificateSigningRequestSpec{
					Request:    req,
					SignerName: "kubernetes.io/kubelet-serving",
					Usages:     []certv1.KeyUsage{"digital signature", "key encipherment", "server auth"},
				},
			}

			fakeClient, ok := client.(*fake.Clientset)
			assert.True(t, ok, "expected Clientset to be of type %T; got %T", &fake.Clientset{}, client)
			fakeClient.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				createAction, ok := action.(k8stesting.CreateActionImpl)
				if !ok {
					return false, nil, fmt.Errorf("expected action to be of type %T; got %T", &k8stesting.CreateActionImpl{}, action)
				}
				sar, ok := createAction.Object.(*authorizationv1.SubjectAccessReview)
				if !ok {
					return false, nil, fmt.Errorf("expected resource to be of type %T; got %T", &authorizationv1.SubjectAccessReview{}, createAction.Object)
				}
				sar.Status.Allowed = true
				return true, sar, nil
			})

			var newCSR *certv1.CertificateSigningRequest
			createCSR := func() {
				t.Helper()
				newCSR, err = client.CertificatesV1().CertificateSigningRequests().Create(ctx, csrReq, metav1.CreateOptions{})
				assert.NoError(t, err)
			}
			if test.startControllerBeforeCreatingCSR {
				assert.NoError(t, c.Start(ctx))
				createCSR()
			} else {
				createCSR()
				assert.NoError(t, c.Start(ctx))
			}

			assert.EventuallyWithT(t, func(c *assert.CollectT) {
				csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, newCSR.Name, metav1.GetOptions{})
				assert.NoError(c, err)
				assert.NotNil(c, csr, "could not find CSR")
				assert.NotEmpty(c, csr.Status.Conditions, "expected to find at least one element in status.conditions")

				for _, condition := range csr.Status.Conditions {
					assert.True(c, condition.Type == certv1.CertificateApproved && condition.Reason == "Autoapproved by K0s CSRApprover" && condition.Status == core.ConditionTrue,
						"expected CSR to be approved")
				}
			}, 2*time.Second, 1*time.Millisecond)

			assert.NoError(t, c.Stop())
		})
	}

}

func pemWithPrivateKey(pk crypto.PrivateKey) []byte {
	template := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   "system:node:worker",
			Organization: []string{"system:nodes"},
		},
		DNSNames: []string{"worker-1"},
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
