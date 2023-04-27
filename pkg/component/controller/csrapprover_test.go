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
	"testing"

	"github.com/k0sproject/k0s/internal/testutil"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/controller/leaderelector"
	"github.com/stretchr/testify/assert"
	certv1 "k8s.io/api/certificates/v1"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBasicCRSApprover(t *testing.T) {
	fakeFactory := testutil.NewFakeClientFactory()

	client, err := fakeFactory.GetClient()
	assert.NoError(t, err)

	ctx := context.TODO()

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

	config := &v1beta1.ClusterConfig{
		Spec: &v1beta1.ClusterSpec{
			API: &v1beta1.APISpec{
				Address:         "1.2.3.4",
				ExternalAddress: "get.k0s.sh",
			},
		},
	}
	c := NewCSRApprover(config, &leaderelector.Dummy{Leader: true}, fakeFactory)

	assert.NoError(t, c.Init(ctx))
	assert.NoError(t, c.approveCSR(ctx))

	csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, newCsr.Name, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, csr)
	assert.True(t, csr.Name == newCsr.Name)
	for _, c := range csr.Status.Conditions {
		assert.True(t, c.Type == certv1.CertificateApproved && c.Reason == "Autoapproved by K0S CSRApprover" && c.Status == core.ConditionTrue)
	}
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
