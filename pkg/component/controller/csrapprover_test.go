// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller_test

import (
	"bytes"
	"cmp"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"net/netip"
	"slices"
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
				{Type: corev1.NodeInternalIP, Address: "FD00:0000:0000:0000:0000:0000:0000:0001"},
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

	validSigningRequest := func() func(template *x509.CertificateRequest) *certv1.CertificateSigningRequest {
		privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		require.NoError(t, err)

		return func(template *x509.CertificateRequest) *certv1.CertificateSigningRequest {
			return &certv1.CertificateSigningRequest{
				ObjectMeta: metav1.ObjectMeta{Name: "csrapprover_test"},
				Spec: certv1.CertificateSigningRequestSpec{
					Request:    pemWithTemplate(template, privateKey),
					SignerName: certv1.KubeletServingSignerName,
					Username:   "system:node:" + node.Name,
					Groups:     []string{"system:nodes"},
					Usages:     []certv1.KeyUsage{certv1.UsageDigitalSignature, certv1.UsageServerAuth},
				},
			}
		}
	}()

	// Returns the test node with additional addresses in its status.
	nodeClaiming := func(addresses ...string) []runtime.Object {
		claiming := node.DeepCopy()
		for _, address := range addresses {
			addressType := corev1.NodeInternalDNS
			if net.ParseIP(address) != nil {
				addressType = corev1.NodeInternalIP
			}
			claiming.Status.Addresses = append(claiming.Status.Addresses, corev1.NodeAddress{Type: addressType, Address: address})
		}
		return []runtime.Object{claiming}
	}

	type reviewMode uint8
	const (
		reviewModeAllow reviewMode = iota
		reviewModeNone
		reviewModeFail
	)

	for _, tt := range []struct {
		testCase string

		// The following things get defaulted to valid values if zero
		username   string
		groups     []string
		template   *x509.CertificateRequest
		csr        *certv1.CertificateSigningRequest // takes precedence over template
		reviewMode reviewMode
		objects    []runtime.Object
		network    *v1beta1.Network

		expectedLog string
		expectedErr string
	}{
		{
			testCase: "node requesting its own certificate",
		},
		{
			testCase: "node requesting its own certificate with non-canonical IP notation",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, netip.AddrFrom16([16]byte{0: 0xfd, 15: 1}).AsSlice())
				return t
			}(),
		},
		{
			testCase: "node reporting 4096 addresses",
			objects: func() []runtime.Object {
				addresses := make([]string, 4096-len(node.Status.Addresses))
				for i := range addresses {
					addresses[i] = fmt.Sprintf("worker-%04d", i)
				}
				return nodeClaiming(addresses...)
			}(),
		},
		{
			testCase: "node reporting 4097 addresses",
			objects: func() []runtime.Object {
				addresses := make([]string, 4097-len(node.Status.Addresses))
				for i := range addresses {
					addresses[i] = fmt.Sprintf("worker-%04d", i)
				}
				return nodeClaiming(addresses...)
			}(),
			expectedErr: "node reports more than 4096 addresses",
		},
		{
			testCase: "certificate request of 512 KiB",
			csr: func() *certv1.CertificateSigningRequest {
				csr := validSigningRequest(validTemplate())
				csr.Spec.Request = append(csr.Spec.Request, bytes.Repeat([]byte{'\n'}, (512*1024)-len(csr.Spec.Request))...)
				return csr
			}(),
		},
		{
			testCase: "certificate request of 512 KiB plus one byte",
			csr: func() *certv1.CertificateSigningRequest {
				csr := validSigningRequest(validTemplate())
				csr.Spec.Request = append(csr.Spec.Request, bytes.Repeat([]byte{'\n'}, (512*1024)+1-len(csr.Spec.Request))...)
				return csr
			}(),
			expectedErr: "certificate request size exceeds 512 KiB",
		},
		{
			testCase: "certificate with 4096 SANs",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, slices.Repeat([]string{t.DNSNames[0]}, 2048-len(t.DNSNames))...)
				t.IPAddresses = append(t.IPAddresses, slices.Repeat([]net.IP{t.IPAddresses[0]}, 2048-len(t.IPAddresses))...)
				return t
			}(),
		},
		{
			testCase: "certificate with 4097 SANs",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, slices.Repeat([]string{t.DNSNames[0]}, 2049-len(t.DNSNames))...)
				t.IPAddresses = append(t.IPAddresses, slices.Repeat([]net.IP{t.IPAddresses[0]}, 2048-len(t.IPAddresses))...)
				return t
			}(),
			expectedErr: "certificate request contains more than 4096 SANs",
		},
		{
			testCase:    "SubjectAccessReview fails",
			reviewMode:  reviewModeFail,
			expectedLog: `Failed to process CSR "csrapprover_test"`,
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
		{
			testCase:    "bootstrap identity requesting a node certificate",
			username:    "system:bootstrap:abcdef",
			groups:      []string{"system:bootstrappers", "system:authenticated"},
			expectedErr: `not requested by a node identity: user "system:bootstrap:abcdef" in group(s) ["system:bootstrappers" "system:authenticated"]`,
		},
		{
			testCase:    "empty requesting node name",
			username:    "system:node:",
			expectedErr: `requesting node name "" is invalid: a lowercase RFC 1123 subdomain must consist of`,
		},
		{
			testCase:    "node user name without system:nodes group",
			groups:      []string{"system:authenticated"},
			expectedErr: `not requested by a node identity: user "system:node:csr-approver-test-node" in group(s) ["system:authenticated"]`,
		},
		{
			testCase:    "node requesting a certificate for another node",
			username:    "system:node:unrelated-worker",
			expectedErr: `requested subject's common name "system:node:csr-approver-test-node" doesn't match requesting user name "system:node:unrelated-worker"`,
		},
		{
			testCase:    "certificate for a non-existent node",
			objects:     []runtime.Object{},
			expectedErr: `nodes "csr-approver-test-node" not found`,
		},
		{
			testCase: "certificate with a foreign DNS name",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "example.com")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "example.com": is not a node address of csr-approver-test-node`,
		},
		{
			testCase: "certificate with a foreign IP address",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, net.IP{192, 0, 2, 1})
				return t
			}(),
			expectedErr: `IPAddresses[1]: forbidden: "192.0.2.1": is not a node address of csr-approver-test-node`,
		},
		{
			testCase: "node with a name that merely resembles the cluster domain",
			objects:  nodeClaiming("node.mycluster.local"),
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "node.mycluster.local")
				return t
			}(),
		},
		// Names and addresses that belong to the control plane or in-cluster
		// services are rejected regardless of what the node reports for itself.
		{
			testCase: "certificate for the API server's cluster IP",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, net.IP{10, 96, 0, 1})
				return t
			}(),
			expectedErr: `IPAddresses[1]: forbidden: "10.96.0.1": is inside a cluster service CIDR`,
		},
		{
			testCase: "zero-padded service CIDR",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, net.IP{10, 96, 0, 1})
				return t
			}(),
			network: func() *v1beta1.Network {
				n := v1beta1.DefaultNetwork()
				n.ServiceCIDR = "10.96.0.0/012"
				return n
			}(),
			expectedErr: `IPAddresses[1]: forbidden: "10.96.0.1": is inside a cluster service CIDR`,
		},
		{
			testCase: "certificate for the API server's IPv6 cluster IP",
			network: func() *v1beta1.Network {
				n := v1beta1.DefaultNetwork()
				n.DualStack = v1beta1.DualStack{Enabled: true, IPv6PodCIDR: "fd00:10:244::/64", IPv6ServiceCIDR: "fd00:10:96::/108"}
				return n
			}(),
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, net.IP{0: 0xfd, 3: 0x10, 5: 0x96, 15: 1})
				return t
			}(),
			expectedErr: `IPAddresses[1]: forbidden: "fd00:10:96::1": is inside a cluster service CIDR`,
		},
		{
			testCase: "certificate for the API server's service name",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "kubernetes.default.svc")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "kubernetes.default.svc": reserved for in-cluster services`,
		},
		{
			testCase: "certificate for the API server's short service name",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "kubernetes")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "kubernetes": reserved for in-cluster services`,
		},
		{
			testCase: "certificate for the API server's cluster IP with the service CIDR in IPv4-mapped notation",
			objects:  nodeClaiming("10.96.0.1"),
			network: func() *v1beta1.Network {
				n := v1beta1.DefaultNetwork()
				n.ServiceCIDR = "::ffff:10.96.0.0/108"
				return n
			}(),
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.IPAddresses = append(t.IPAddresses, net.IP{10, 96, 0, 1})
				return t
			}(),
			expectedErr: `IPAddresses[1]: forbidden: "10.96.0.1": is inside a cluster service CIDR`,
		},
		{
			testCase: "certificate for the API server's service name in non-canonical notation",
			objects:  nodeClaiming("Kubernetes.Default."),
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "Kubernetes.Default.")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "Kubernetes.Default.": reserved for in-cluster services`,
		},
		{
			testCase: "certificate for another service's name",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "metrics-server.kube-system.svc")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "metrics-server.kube-system.svc": reserved for in-cluster services`,
		},
		{
			testCase: "certificate for a name in the cluster domain",
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "foo.bar.cluster.local")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "foo.bar.cluster.local": reserved for in-cluster services`,
		},
		{
			testCase: "certificate for a name in a custom cluster domain",
			network: func() *v1beta1.Network {
				n := v1beta1.DefaultNetwork()
				n.ClusterDomain = "example.com"
				return n
			}(),
			template: func() *x509.CertificateRequest {
				t := validTemplate()
				t.DNSNames = append(t.DNSNames, "foo.bar.example.com")
				return t
			}(),
			expectedErr: `DNSNames[1]: forbidden: "foo.bar.example.com": reserved for in-cluster services`,
		},
	} {
		t.Run(tt.testCase, func(t *testing.T) {
			if tt.username == "" {
				tt.username = "system:node:csr-approver-test-node"
			}
			if tt.groups == nil {
				tt.groups = []string{"system:nodes"}
			}
			if tt.objects == nil {
				tt.objects = []runtime.Object{node}
			}
			if tt.network == nil {
				tt.network = v1beta1.DefaultNetwork()
			}

			synctest.Test(t, func(t *testing.T) {
				fakeFactory := testutil.NewFakeClientFactory(tt.objects...)
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

				csr := tt.csr
				if tt.csr == nil {
					template := tt.template
					if template == nil {
						template = validTemplate()
					}
					csr = validSigningRequest(template)
				}
				csr.Spec.Username, csr.Spec.Groups = tt.username, tt.groups

				_, err := client.CertificatesV1().CertificateSigningRequests().Create(t.Context(), csr, metav1.CreateOptions{})
				require.NoError(t, err)

				logger, logs := test.NewNullLogger()
				ctx := k0scontext.WithValue[logrus.FieldLogger](t.Context(), logger)
				underTest := controller.NewCSRApprover(leaderelector.Off(), fakeFactory, tt.network)
				require.NoError(t, underTest.Init(ctx))
				require.NoError(t, underTest.Start(ctx))
				t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

				synctest.Wait()

				csr, err = client.CertificatesV1().CertificateSigningRequests().Get(t.Context(), csr.Name, metav1.GetOptions{})
				require.NoError(t, err)

				var expectedApproved corev1.ConditionStatus
				if tt.expectedErr == "" {
					expectedApproved = corev1.ConditionTrue
				}
				assertApprovedStatus(t, expectedApproved, csr)

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

	t.Run("handles all pending requests in one pass", func(t *testing.T) {
		var objects []runtime.Object
		for i := range 3 {
			node := node.DeepCopy()
			node.Name = fmt.Sprintf("csr-test-node-%d", i)

			template := validTemplate()
			template.Subject.CommonName = "system:node:" + node.Name
			csr := validSigningRequest(template)
			csr.Name = fmt.Sprintf("csr-approver-test-%d", i)
			csr.Spec.Username = template.Subject.CommonName

			objects = append(objects, node, csr)
		}

		synctest.Test(t, func(t *testing.T) {
			fakeFactory := testutil.NewFakeClientFactory(objects...)
			client := fakeFactory.Client.(*kubernetesfake.Clientset)

			client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				sar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				sar.Status.Allowed = true
				return true, sar, nil
			})

			underTest := controller.NewCSRApprover(leaderelector.Off(), fakeFactory, v1beta1.DefaultNetwork())
			require.NoError(t, underTest.Init(t.Context()))
			require.NoError(t, underTest.Start(t.Context()))
			t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

			synctest.Wait()

			csrs, err := client.CertificatesV1().CertificateSigningRequests().List(t.Context(), metav1.ListOptions{})
			require.NoError(t, err)
			require.Len(t, csrs.Items, 3)
			for i := range csrs.Items {
				assertApprovedStatus(t, corev1.ConditionTrue, &csrs.Items[i])
			}
		})
	})

	t.Run("handles newest request per node first", func(t *testing.T) {
		// Lists are sorted by name. The newest request is in the middle, so
		// that neither the first nor the last request in the list is the one to
		// be picked.
		now := metav1.Now()
		csr1 := validSigningRequest(validTemplate())
		csr1.Name, csr1.CreationTimestamp = "csr-1", metav1.NewTime(now.Add(-2*time.Hour))
		csr2 := csr1.DeepCopy()
		csr2.Name, csr2.CreationTimestamp = "csr-2", now
		csr3 := csr1.DeepCopy()
		csr3.Name, csr3.CreationTimestamp = "csr-3", metav1.NewTime(now.Add(-1*time.Hour))

		synctest.Test(t, func(t *testing.T) {
			fakeFactory := testutil.NewFakeClientFactory(node, csr1, csr2, csr3)
			client := fakeFactory.Client.(*kubernetesfake.Clientset)

			client.PrependReactor("create", "subjectaccessreviews", func(action k8stesting.Action) (bool, runtime.Object, error) {
				sar := action.(k8stesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
				sar.Status.Allowed = true
				return true, sar, nil
			})

			underTest := controller.NewCSRApprover(leaderelector.Off(), fakeFactory, v1beta1.DefaultNetwork())
			require.NoError(t, underTest.Init(t.Context()))
			require.NoError(t, underTest.Start(t.Context()))
			t.Cleanup(func() { assert.NoError(t, underTest.Stop()) })

			// Expect three ticks, csr2 first, then csr3, then csr1.
			for i, tick := range [][3]bool{
				{false, true, false},
				{false, true, true},
				{true, true, true},
			} {
				synctest.Wait()
				t.Log("Tick", i+1)

				csrs, err := client.CertificatesV1().CertificateSigningRequests().List(t.Context(), metav1.ListOptions{})
				if assert.NoError(t, err) && assert.Len(t, csrs.Items, len(tick)) {
					for i, expected := range tick {
						var expectedStatus corev1.ConditionStatus
						if expected {
							expectedStatus = corev1.ConditionTrue
						}
						assertApprovedStatus(t, expectedStatus, &csrs.Items[i])
					}
				}

				time.Sleep(12 * time.Second)
			}
		})
	})
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

func assertApprovedStatus(t *testing.T, expected corev1.ConditionStatus, csr *certv1.CertificateSigningRequest) {
	t.Helper()

	var approvedCond *certv1.CertificateSigningRequestCondition
	for _, c := range csr.Status.Conditions {
		if c.Type == certv1.CertificateApproved {
			approvedCond = &c
			break
		}
	}
	if expected == "" {
		assert.Nilf(t, approvedCond, "Expected no approved condition at all for %s", csr.Name)
	} else if assert.NotNilf(t, approvedCond, "Expected an approved condition for %s", csr.Name) {
		assert.Equalf(t, expected, approvedCond.Status, "Unexpected status in approved condition for %s: %v", csr.Name, approvedCond)
	}
}
