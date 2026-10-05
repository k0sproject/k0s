// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package kubeletcertrotate

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/transport"

	"github.com/k0sproject/k0s/inttest/common"
	"github.com/stretchr/testify/suite"
)

type kubeletCertRotateSuite struct {
	common.BootlooseSuite
}

const (
	// The TTL that kube-controller-manager's CSR signer is configured with.
	// Smaller values might otherwise get rotated on the spot.
	signingDuration = 3 * time.Minute

	// NotBefore gets backdated by this amount.
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/controller/certificates/signer/signer.go#L209
	notBeforeBackdate = 5 * time.Minute

	// The expected lifetime duration for certificates.
	lifetime = signingDuration + notBeforeBackdate
)

// SetupTest prepares the controller and filesystem, getting it into a consistent
// state which we can run tests against.
func (s *kubeletCertRotateSuite) SetupTest() {
	s.Require().NoError(s.InitController(0, "--disable-components=metrics-server", fmt.Sprintf("--kube-controller-manager-extra-args='--cluster-signing-duration=%s'", signingDuration)))
	s.Require().NoError(s.WaitJoinAPI(s.ControllerNode(0)))

	// Create a worker join token
	workerJoinToken, err := s.GetJoinToken("worker")
	s.Require().NoError(err)

	// Start the workers using the join token
	s.Require().NoError(s.RunWorkersWithToken(workerJoinToken, "--kubelet-root-dir=/var/lib/kubelet"))

	client, err := s.KubeClient(s.ControllerNode(0))
	s.Require().NoError(err)

	for idx := range s.WorkerCount {
		s.Require().NoError(s.WaitForNodeReady(s.WorkerNode(idx), client))
	}
}

func (s *kubeletCertRotateSuite) TestWorkerClientsSurviveRotation() {
	ctx := s.Context()

	// Knowing that `kube-controller-manager` is issuing short-lived
	// certificates, if the worker clients can still talk to the API server
	// *after* the kubelet has rotated its key/cert *and* the initial
	// certificate has expired, we should be able to confidently say that the
	// transport cert rotation is fine. The status socket's worker connectivity
	// probe is used for this, as it shares the client factory, and hence the
	// cached client-go transport, with all the other worker-side clients.
	workerSSH, err := s.SSH(ctx, s.WorkerNode(0))
	s.Require().NoError(err)

	// Probe once before the rotation, so that the status component's client
	// and its connection to the API server predate the rotation.
	success, message := s.probeWorkerToAPIConnection(ctx, workerSSH)
	s.Require().Truef(success, "Worker-to-API probe failed before the rotation: %s", message)
	s.T().Log("Worker-to-API probe succeeded before the rotation")

	// Rotation happens after 70-90% of the certificate lifetime.
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/staging/src/k8s.io/client-go/util/certificate/certificate_manager.go#L736
	initial := s.readKubeletClientCert(ctx, workerSSH)
	earliestRotation := /* 70%: */ initial.NotBefore.Add(7 * lifetime / 10)
	latestRotation := /*   90%: */ initial.NotBefore.Add(9 * lifetime / 10)

	deadline := latestRotation.Add(30 * time.Second) // slack for the CSR round-trip
	s.T().Log("Waiting for the kubelet to rotate the initial client cert until", deadline.Format(time.TimeOnly))

	var rotated *x509.Certificate
	for deadline := time.After(time.Until(deadline)); rotated == nil || rotated.SerialNumber.Cmp(initial.SerialNumber) == 0; {
		select {
		case <-time.After(5 * time.Second):
			rotated = s.readKubeletClientCert(ctx, workerSSH)
		case <-deadline:
			s.Require().Fail("Kubelet didn't rotate its client certificate in time")
		case <-ctx.Done():
			s.Require().Fail("Test interrupted")
		}
	}

	issuedAt := rotated.NotBefore.Add(notBeforeBackdate)
	s.T().Log("Kubelet rotated its client cert at", issuedAt.Format(time.TimeOnly))
	s.Require().Falsef(
		issuedAt.Before(earliestRotation),
		"Kubelet rotated its client certificate too early: %s before %s",
		issuedAt.Format(time.TimeOnly),
		earliestRotation.Format(time.TimeOnly))
	s.Require().Falsef(
		issuedAt.After(deadline),
		"Kubelet rotated its client certificate too late: %s after %s",
		issuedAt.Format(time.TimeOnly),
		deadline.Format(time.TimeOnly))

	// The initial certificate stays valid for a while after the rotation, and
	// the API server verifies the client certificate on every request, not just
	// during the TLS handshake:
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/staging/src/k8s.io/apiserver/pkg/authentication/request/x509/x509.go#L194
	// A request over a connection that's still presenting the expired
	// certificate gets rejected, rather than being treated as anonymous:
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/pkg/kubeapiserver/authenticator/config.go#L245
	// So wait for the initial certificate to expire before probing again. A
	// successful probe then proves that the new certificate is in use.
	deadline = initial.NotAfter.Add(1 * time.Second)
	s.T().Log("Waiting until", deadline.Format(time.TimeOnly), "for the initial client cert to expire")
	select {
	case <-time.After(time.Until(deadline)):
	case <-ctx.Done():
		s.Require().Fail("Test interrupted")
	}

	// The worker clients rely on client-go to reload the certificate files, and
	// to close connections that have been established using the initial
	// certificate. It reloads the files during each TLS handshake ...
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/staging/src/k8s.io/client-go/transport/cache.go#L135
	// ... and on a timer (see CertCallbackRefreshDuration below). A new
	// handshake only happens if there's no usable connection in the pool, and
	// the connection is usually kept busy by other clients that share the
	// transport. The first probe after the expiry is hence expected to fail.
	// The API server then tears down the HTTP/2 connection along with the
	// rejected request:
	// https://github.com/kubernetes/kubernetes/blob/v1.37.0/staging/src/k8s.io/apiserver/pkg/endpoints/filters/authentication.go#L139
	// The next probe dials a new connection, and the handshake picks up the new
	// certificate. Should this not happen for whatever reason, the timer is the
	// fallback, so that's the upper bound for the recovery.
	deadline = initial.NotAfter.Add(transport.CertCallbackRefreshDuration + 5*time.Second)
	s.T().Log("Waiting for the worker-to-API probe to succeed until", deadline.Format(time.TimeOnly))
	for failureSeen, deadline := false, time.After(time.Until(deadline)); ; {
		success, message := s.probeWorkerToAPIConnection(ctx, workerSSH)
		if success {
			if failureSeen {
				s.T().Log("Worker-to-API probe succeeded",
					time.Since(initial.NotAfter).Round(time.Second),
					"after the initial client cert expired")
				break
			}
			if time.Since(initial.NotAfter) >= 10*time.Second {
				s.T().Log("Worker-to-API probe succeeded 10 seconds after the initial cert expired")
				break
			}
		} else {
			s.Require().Equal("the server has asked for the client to provide credentials", message)
			if !failureSeen {
				s.T().Log("Worker-to-API probe failed because the initial client cert expired")
				failureSeen = true
			}
		}

		select {
		case <-time.After(1 * time.Second):
		case <-deadline:
			s.Require().Fail("Worker-to-API probe didn't recover in time after the initial client cert expired")
		case <-ctx.Done():
			s.Require().Fail("Test interrupted")
		}
	}
}

// probeWorkerToAPIConnection queries the worker's status socket and returns
// the outcome of its worker-to-API connection probe.
func (s *kubeletCertRotateSuite) probeWorkerToAPIConnection(ctx context.Context, workerSSH *common.SSHConnection) (success bool, message string) {
	output, err := workerSSH.ExecWithOutput(ctx, "k0s status -ojson")
	s.Require().NoError(err)
	var status map[string]any
	s.Require().NoError(json.Unmarshal([]byte(output), &status))

	success, found, err := unstructured.NestedBool(status, "WorkerToAPIConnectionStatus", "Success")
	s.Require().NoError(err)
	s.Require().True(found, "Worker-to-API connection status not found in status output")
	message, _, err = unstructured.NestedString(status, "WorkerToAPIConnectionStatus", "Message")
	s.Require().NoError(err)
	return success, message
}

func (s *kubeletCertRotateSuite) readKubeletClientCert(ctx context.Context, workerSSH *common.SSHConnection) *x509.Certificate {
	var buf bytes.Buffer
	err := workerSSH.Exec(ctx, "cat /var/lib/kubelet/pki/kubelet-client-current.pem", common.SSHStreams{
		Out: &buf,
	})
	s.Require().NoError(err)

	for block, rest := pem.Decode(buf.Bytes()); block != nil; block, rest = pem.Decode(rest) {
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		s.Require().NoError(err)
		certLifetime := cert.NotAfter.Sub(cert.NotBefore)
		s.Require().Equal(lifetime, certLifetime, "Unexpected kubelet client certificate lifetime")
		return cert
	}

	s.Require().FailNow("No certificate found in /var/lib/kubelet/pki/kubelet-client-current.pem")
	return nil
}

func TestKubeletCertRotateSuite(t *testing.T) {
	suite.Run(t, &kubeletCertRotateSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
			LaunchMode:      common.LaunchModeOpenRC,
		},
	})
}
