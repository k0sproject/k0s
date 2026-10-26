// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package clusterinfodiscovery

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	bootstrapapi "k8s.io/cluster-bootstrap/token/api"
	"k8s.io/kubernetes/cmd/kubeadm/app/constants"

	"github.com/stretchr/testify/suite"
)

const (
	clusterInfoPath = "/api/v1/namespaces/kube-public/configmaps/" + bootstrapapi.ConfigMapClusterInfo
	tokenID         = "abcdef"
	tokenSecret     = "0123456789abcdef"
)

type ClusterInfoDiscoverySuite struct {
	common.BootlooseSuite
}

func (s *ClusterInfoDiscoverySuite) TestClusterInfoDiscovery() {
	ctx := s.Context()
	controller := s.ControllerNode(0)

	config := func(enabled bool) string {
		return fmt.Sprintf("spec:\n  api:\n    clusterInfoDiscovery: %t\n", enabled)
	}
	// Clusters start without it, as existing clusters do.
	s.PutFile(controller, "/tmp/k0s.yaml", config(false))
	s.Require().NoError(s.InitController(0, "--config=/tmp/k0s.yaml"))

	restConfig, err := s.GetKubeConfig(controller)
	s.Require().NoError(err)
	kc, err := s.KubeClient(controller)
	s.Require().NoError(err)

	anonymous := rest.AnonymousClientConfig(restConfig)
	// Bootstrap tokens authenticate as members of the group that joining nodes use.
	bootstrapToken := rest.AnonymousClientConfig(restConfig)
	bootstrapToken.BearerToken = tokenID + "." + tokenSecret
	_, err = kc.CoreV1().Secrets(metav1.NamespaceSystem).Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: bootstrapapi.BootstrapTokenSecretPrefix + tokenID},
		Type:       bootstrapapi.SecretTypeBootstrapToken,
		StringData: map[string]string{
			bootstrapapi.BootstrapTokenIDKey:               tokenID,
			bootstrapapi.BootstrapTokenSecretKey:           tokenSecret,
			bootstrapapi.BootstrapTokenUsageAuthentication: "true",
			bootstrapapi.BootstrapTokenExtraGroupsKey:      constants.NodeBootstrapTokenAuthGroup,
			bootstrapapi.BootstrapTokenDescriptionKey:      "cluster-info discovery test",
			bootstrapapi.BootstrapTokenUsageSigningKey:     "true",
			bootstrapapi.BootstrapTokenExpirationKey:       time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	s.Run("disabled by default", func() {
		s.assertDisabled(ctx, kc, anonymous, bootstrapToken)
	})

	s.Run("enabling", func() {
		// The setting is part of the node-local configuration, so it changes with a restart.
		s.PutFile(controller, "/tmp/k0s.yaml", config(true))
		s.Require().NoError(s.RestartController(controller))
		s.Require().NoError(s.WaitForKubeAPI(controller))
	})

	s.Run("anonymous cluster-info", func() {
		var body []byte
		s.Require().NoError(wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			var status int
			status, body, err = get(ctx, anonymous, clusterInfoPath)
			return err == nil && status == http.StatusOK, nil
		}), "anonymous requests should get the cluster-info")

		var clusterInfo corev1.ConfigMap
		s.Require().NoError(json.Unmarshal(body, &clusterInfo))
		var kubeconfig *clientcmdapi.Config
		kubeconfig, err = clientcmd.Load([]byte(clusterInfo.Data[bootstrapapi.KubeConfigKey]))
		s.Require().NoError(err)
		s.Require().Len(kubeconfig.Clusters, 1)
		var server *url.URL
		for _, cluster := range kubeconfig.Clusters {
			s.Equal(restConfig.CAData, cluster.CertificateAuthorityData, "the cluster CA is published")
			server, err = url.Parse(cluster.Server)
			s.Require().NoError(err)
			s.Equal("https", server.Scheme)
			s.Equal("6443", server.Port())
		}
		s.Empty(kubeconfig.AuthInfos, "no credentials are published")
	})

	s.Run("anonymous requests", func() {
		// Anonymous requests only authenticate for the cluster-info.
		for _, path := range otherAnonymousPaths {
			var status int
			status, _, err = get(ctx, anonymous, path)
			if s.NoError(err, path) {
				s.Equal(http.StatusUnauthorized, status, path)
			}
		}
	})

	s.Run("bootstrap token requests", func() {
		// Joining nodes read the cluster and kubelet configuration, and check for nodes with their name.
		// They read the cluster-info anonymously before, so the token itself doesn't need it.
		statuses := map[string]int{}
		s.Require().NoError(wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
			readable := true
			for _, path := range []string{
				"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeadmConfigConfigMap,
				"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeletBaseConfigurationConfigMap,
			} {
				var status int
				status, _, err = get(ctx, bootstrapToken, path)
				statuses[path] = status
				readable = readable && err == nil && status == http.StatusOK
			}
			return readable, nil
		}), "bootstrap tokens should read what joining nodes need, last statuses %v", statuses)
		var status int
		status, _, err = get(ctx, bootstrapToken, "/api/v1/nodes/nonexistent")
		if s.NoError(err) {
			s.Equal(http.StatusNotFound, status, "bootstrap tokens may get nodes")
		}

		for _, path := range []string{
			"/api/v1/nodes",
			"/api/v1/namespaces/kube-system/configmaps",
			"/api/v1/namespaces/kube-system/configmaps/coredns",
			"/api/v1/namespaces/kube-system/secrets",
		} {
			status, _, err = get(ctx, bootstrapToken, path)
			if s.NoError(err, path) {
				s.Equal(http.StatusForbidden, status, path)
			}
		}
	})

	s.Run("disabling", func() {
		s.PutFile(controller, "/tmp/k0s.yaml", config(false))
		s.Require().NoError(s.RestartController(controller))
		s.Require().NoError(s.WaitForKubeAPI(controller))
		s.assertDisabled(ctx, kc, anonymous, bootstrapToken)
	})
}

// Paths that anonymous requests never authenticate for, not even while enabled.
var otherAnonymousPaths = []string{
	"/version",
	"/api/v1/nodes",
	"/api/v1/namespaces/kube-public/configmaps",
	"/api/v1/namespaces/kube-public/configmaps?watch=true",
	"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeadmConfigConfigMap,
	"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeletBaseConfigurationConfigMap,
}

// assertDisabled waits until nothing is published, anonymous requests don't authenticate at all,
// and bootstrap tokens get none of what joining nodes read.
func (s *ClusterInfoDiscoverySuite) assertDisabled(ctx context.Context, kc kubernetes.Interface, anonymous, bootstrapToken *rest.Config) {
	type request struct {
		restConfig *rest.Config
		path       string
		want       int
	}
	tokenPaths := []string{
		"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeadmConfigConfigMap,
		"/api/v1/namespaces/kube-system/configmaps/" + constants.KubeletBaseConfigurationConfigMap,
		"/api/v1/nodes/nonexistent",
	}
	requests := make([]request, 0, 1+len(otherAnonymousPaths)+len(tokenPaths))
	requests = append(requests, request{anonymous, clusterInfoPath, http.StatusUnauthorized})
	for _, path := range otherAnonymousPaths {
		requests = append(requests, request{anonymous, path, http.StatusUnauthorized})
	}
	for _, path := range tokenPaths {
		requests = append(requests, request{bootstrapToken, path, http.StatusForbidden})
	}

	unexpected := map[string]string{}
	s.Require().NoError(wait.PollUntilContextTimeout(ctx, 2*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		clear(unexpected)
		var err error
		for _, configMap := range []struct{ namespace, name string }{
			{metav1.NamespacePublic, bootstrapapi.ConfigMapClusterInfo},
			{metav1.NamespaceSystem, constants.KubeadmConfigConfigMap},
			{metav1.NamespaceSystem, constants.KubeletBaseConfigurationConfigMap},
		} {
			_, err = kc.CoreV1().ConfigMaps(configMap.namespace).Get(ctx, configMap.name, metav1.GetOptions{})
			if !apierrors.IsNotFound(err) {
				unexpected[configMap.namespace+"/"+configMap.name] = fmt.Sprintf("exists or %v", err)
			}
		}
		for _, r := range requests {
			var status int
			status, _, err = get(ctx, r.restConfig, r.path)
			if err != nil || status != r.want {
				unexpected[r.path] = fmt.Sprintf("status %d, want %d, %v", status, r.want, err)
			}
		}
		return len(unexpected) == 0, nil
	}), "nothing should be published or accessible, unexpected: %v", unexpected)
}

// get requests the path with the client configuration and returns the status code and body.
func get(ctx context.Context, restConfig *rest.Config, path string) (int, []byte, error) {
	client, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return 0, nil, err
	}
	server, _, err := rest.DefaultServerUrlFor(restConfig)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(server.String(), "/")+path, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, body, err
}

func TestClusterInfoDiscoverySuite(t *testing.T) {
	suite.Run(t, &ClusterInfoDiscoverySuite{
		common.BootlooseSuite{ControllerCount: 1},
	})
}
