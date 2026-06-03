// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0
package containerdupgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/k0sproject/k0s/inttest/common"
	kubeutil "github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/kubernetes/watch"
	"github.com/k0sproject/version"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
)

type ContainerdUpgradeSuite struct {
	common.BootlooseSuite
}

// TODO: Remove this test in k0s 1.37+

func (s *ContainerdUpgradeSuite) TestContainerdUpgrade() {
	ctx := s.Context()
	var kc *kubernetes.Clientset
	var oldPIDs map[string]int

	nodeName := s.WorkerNode(0)
	s.Run("controller_and_workers_get_up", func() {
		s.Require().NoError(s.InitController(0))
		var err error
		kc, err = s.KubeClient(s.ControllerNode(0))
		s.Require().NoError(err)
		// Because we launch k0s control plane with 1.36, the 1.35 worker profile won't be created automatically.
		// This is a hack but it's much faster and works for the test.
		s.create1_35WorkerProfile(ctx, kc)
		s.create1_35WorkerProfileRBAC(ctx, kc)

		// get the latest 1.35 stable release so that we start the node with containerd 1.7.x
		s.downloadRelease(ctx, nodeName, s.getLast35Release(ctx))
		s.Require().NoError(s.RunWorkers())

		s.Require().NoError(s.WaitForNodeReady(nodeName, kc))
		actual := s.getContainerdVersion(ctx, nodeName)
		s.Require().Truef(strings.HasPrefix(actual, "1.7."), "Expected containerd 1.7.x, got %s", actual)
	})

	s.Run("launch_test_pods", func() {
		for _, podName := range []string{"nginx-kill", "nginx-graceful"} {
			_, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Create(ctx, &corev1.Pod{
				TypeMeta:   metav1.TypeMeta{Kind: "Pod", APIVersion: "v1"},
				ObjectMeta: metav1.ObjectMeta{Name: podName},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "nginx", Image: "docker.io/library/nginx:1.31.1-alpine"}},
				},
			}, metav1.CreateOptions{})
			s.Require().NoError(err)
		}
	})

	s.Run("gather_pids_before_upgrade", func() {
		s.Require().NoError(common.WaitForPod(ctx, kc, "nginx-kill", metav1.NamespaceDefault), "nginx pod did not start")
		s.Require().NoError(common.WaitForPod(ctx, kc, "nginx-graceful", metav1.NamespaceDefault), "nginx pod did not start")
		oldPIDs = s.gatherPIDs(ctx, nodeName)
	})

	s.Run("add_containerd_v2_config", func() {
		s.PutFile(nodeName, "/etc/k0s/containerd.d/v2.toml", "version = 2")
	})

	s.Run("upgrade_k0s_to_testing_version", func() {
		expected := getContainerdVersion(s.T())
		s.Require().NoError(s.StopWorker(nodeName))
		s.upgradeContainerdToTesting(ctx, nodeName)
		// k0s should exit after containerd upgrade because of the containerd v2 config
		s.validateK0sExitAndCleanUpContainerdV2Conf(ctx, nodeName)

		// After removing it, it should start up again
		s.Require().NoError(s.StartWorker(nodeName))

		// Wait for the kubelet to re-appear.
		var lastRenewTime *metav1.MicroTime
		s.Require().NoError(watch.FromClient[*coordinationv1.LeaseList, coordinationv1.Lease](kc.CoordinationV1().Leases(corev1.NamespaceNodeLease)).
			WithObjectName(nodeName).
			WithErrorCallback(common.RetryWatchErrors(s.T().Logf)).
			Until(ctx, func(lease *coordinationv1.Lease) (done bool, err error) {
				if lease.Spec.RenewTime == nil || !kubeutil.IsValidLease(*lease) {
					s.T().Logf("Lease is invalid: %#v", *lease)
					return false, nil
				}

				if lastRenewTime == nil {
					lastRenewTime = lease.Spec.RenewTime
					s.T().Logf("Waiting for renewal after %v", lastRenewTime)
					return false, nil
				}

				if !lastRenewTime.Before(lease.Spec.RenewTime) {
					s.T().Logf("Still waiting for renewal after %v", lastRenewTime)
					return false, nil
				}

				return true, nil
			}))

		actual := s.getContainerdVersion(ctx, nodeName)
		s.Require().Equal(expected, actual, "Unexpected containerd version after upgrade")
	})

	s.Run("validate_no_restarts", func() {
		newPIDs := s.gatherPIDs(ctx, nodeName)
		s.Require().Equal(oldPIDs, newPIDs, "PIDs of running containers changed after containerd upgrade")
	})

	s.Run("gracefully_terminate_nginx_pod", func() {
		s.Require().NoError(kc.CoreV1().Pods(metav1.NamespaceDefault).Delete(ctx, "nginx-graceful", metav1.DeleteOptions{}))
		s.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, false, func(ctx context.Context) (bool, error) {
			_, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Get(ctx, "nginx-graceful", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, err
		}))
	})

	s.Run("force_kill_nginx_pod", func() {
		s.forceKillNginx(ctx, nodeName)
		s.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, false, func(ctx context.Context) (bool, error) {
			pod, err := kc.CoreV1().Pods(metav1.NamespaceDefault).Get(ctx, "nginx-kill", metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return pod.Status.ContainerStatuses[0].RestartCount > 0, nil
		}))
	})
}

func (s *ContainerdUpgradeSuite) validateK0sExitAndCleanUpContainerdV2Conf(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	s.T().Log("waiting for k0s process to stop")
	err = wait.PollUntilContextCancel(ctx, 1*time.Second, false, func(ctx context.Context) (bool, error) {
		_, err := ssh.ExecWithOutput(ctx, "pgrep /usr/local/bin/k0s")
		if err != nil {
			return true, nil
		}
		return false, nil
	})
	s.Require().NoError(err)

	s.T().Logf("k0s process exited")

	_, err = ssh.ExecWithOutput(ctx, "rm -f /etc/k0s/containerd.d/v2.toml")
	s.Require().NoError(err)
}

func (s *ContainerdUpgradeSuite) forceKillNginx(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(ctx, "pkill -9 'nginx: master process nginx'")
	s.Require().NoError(err)
}

func (s *ContainerdUpgradeSuite) upgradeContainerdToTesting(ctx context.Context, node string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	_, err = ssh.ExecWithOutput(ctx, "rm -f /usr/local/bin/k0s")
	s.Require().NoError(err)
	_, err = ssh.ExecWithOutput(ctx, "mv /usr/local/bin/k0s-to-test /usr/local/bin/k0s")
	s.Require().NoError(err)
}

func getContainerdVersion(t *testing.T) string {
	cmd := exec.Command("."+string(filepath.Separator)+"vars.sh", "containerd_version")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	require.NoError(t, err)
	version, _, _ := bytes.Cut(out, []byte{'\n'})
	require.NotEmpty(t, version, "Failed to get containerd version")
	return string(version)
}

func (s *ContainerdUpgradeSuite) getContainerdVersion(ctx context.Context, nodeName string) string {
	ssh, err := s.SSH(ctx, nodeName)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "/var/lib/k0s/bin/containerd --version")
	s.Require().NoError(err)
	fields := strings.Fields(output)
	s.Require().GreaterOrEqualf(len(fields), 3, "Not enough fields in containerd --version output: %v", output)
	return fields[2]
}

func (s *ContainerdUpgradeSuite) downloadRelease(ctx context.Context, node string, release string) {
	ssh, err := s.SSH(ctx, node)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	arch, err := ssh.ExecWithOutput(ctx, "uname -m")
	s.Require().NoError(err)
	switch arch {
	case "x86_64":
		arch = "amd64"
	case "aarch64":
		arch = "arm64"
	case "armv7l":
		arch = "arm"
	default:
		s.Failf("unsupported architecture: %s", arch)
	}

	_, err = ssh.ExecWithOutput(ctx, "mv /usr/local/bin/k0s /usr/local/bin/k0s-to-test")
	s.Require().NoError(err)

	url := fmt.Sprintf("https://github.com/k0sproject/k0s/releases/download/%s/k0s-%s-%s", release, release, arch)

	s.T().Logf("Downloading k0s release %s for architecture %s from URL: %s", release, arch, url)
	_, err = ssh.ExecWithOutput(ctx, "wget -qO /usr/local/bin/k0s "+url)
	s.Require().NoError(err)

	_, err = ssh.ExecWithOutput(ctx, "chmod +x /usr/local/bin/k0s")
	s.Require().NoError(err)
}

func (s *ContainerdUpgradeSuite) getLast35Release(ctx context.Context) string {
	type Release struct {
		Name string `json:"name"`
	}

	url := "https://api.github.com/repos/k0sproject/k0s/releases?per_page=10"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	s.Require().NoError(err, "failed to create request")

	resp, err := http.DefaultClient.Do(req)
	s.Require().NoError(err, "failed to fetch releases")
	defer resp.Body.Close()

	s.Require().Equal(http.StatusOK, resp.StatusCode, "unexpected status code from GitHub API")

	var releases []Release
	err = json.NewDecoder(resp.Body).Decode(&releases)
	s.Require().NoError(err, "failed to decode releases response")

	constraint := version.MustConstraint(">=1.35.0, <1.36.1")

	var latestVersion *version.Version
	var latestTag string

	for _, release := range releases {
		ver := version.MustParse(release.Name)
		if !constraint.Check(ver) {
			continue
		}

		if latestVersion == nil || ver.GreaterThan(latestVersion) {
			latestVersion = ver
			latestTag = release.Name
		}
	}

	s.Require().NotEmpty(latestTag, "no 1.35.x release found")
	return latestTag
}

func (s *ContainerdUpgradeSuite) gatherPIDs(ctx context.Context, name string) map[string]int {
	type namespace struct {
		Type string `json:"type"`
		Path string `json:"path,omitempty"`
	}

	type containerInfo struct {
		Labels map[string]string `json:"Labels"`
		Spec   struct {
			Linux struct {
				Namespaces []namespace `json:"namespaces"`
			} `json:"linux"`
		} `json:"Spec"`
	}

	ssh, err := s.SSH(ctx, name)
	s.Require().NoError(err)
	defer ssh.Disconnect()

	output, err := ssh.ExecWithOutput(ctx, "k0s ctr container list --quiet")
	s.Require().NoError(err)

	pidMap := make(map[string]int)

	for _, containerID := range strings.Fields(output) {
		infoOutput, err := ssh.ExecWithOutput(ctx, "k0s ctr container info "+containerID)
		s.Require().NoError(err)

		var info containerInfo
		s.Require().NoError(json.Unmarshal([]byte(infoOutput), &info))

		// We cannot get the PID from sandbox containers
		if info.Labels["io.cri-containerd.kind"] != "container" {
			continue
		}

		podName := info.Labels["io.kubernetes.pod.name"]
		if !strings.HasPrefix(podName, "nginx-") {
			continue
		}
		for _, ns := range info.Spec.Linux.Namespaces {
			if m := regexp.MustCompile(`^/proc/(\d+)/ns/`).FindStringSubmatch(ns.Path); m != nil {
				pid, err := strconv.Atoi(m[1])
				s.Require().NoError(err)
				pidMap[podName] = pid
				break
			}
		}
	}
	return pidMap
}

func (s *ContainerdUpgradeSuite) create1_35WorkerProfile(ctx context.Context, kc *kubernetes.Clientset) {
	var cm *corev1.ConfigMap
	s.Require().NoError(wait.PollUntilContextCancel(ctx, 1*time.Second, true, func(ctx context.Context) (bool, error) {
		var err error
		cm, err = kc.CoreV1().ConfigMaps(metav1.NamespaceSystem).Get(ctx, "worker-config-default-1.36", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return err == nil, err
	}))

	newData := make(map[string]string, len(cm.Data))
	for k, v := range cm.Data {
		newData[k] = strings.ReplaceAll(v, "1.36", "1.35")
	}

	newLabels := make(map[string]string, len(cm.Labels))
	for k, v := range cm.Labels {
		newLabels[k] = strings.ReplaceAll(v, "1.36", "1.35")
	}

	_, err := kc.CoreV1().ConfigMaps(metav1.NamespaceSystem).Create(ctx, &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-config-default-1.35",
			Namespace: metav1.NamespaceSystem,
			Labels:    newLabels,
		},
		Data: newData,
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func (s *ContainerdUpgradeSuite) create1_35WorkerProfileRBAC(ctx context.Context, kc *kubernetes.Clientset) {
	_, err := kc.RbacV1().Roles(metav1.NamespaceSystem).Create(ctx, &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system:bootstrappers:worker-config-1.35",
			Namespace: metav1.NamespaceSystem,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				ResourceNames: []string{"worker-config-default-1.35"},
				Resources:     []string{"configmaps"},
				Verbs:         []string{"get", "list", "watch"},
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)

	_, err = kc.RbacV1().RoleBindings(metav1.NamespaceSystem).Create(ctx, &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "system:bootstrappers:worker-config-1.35",
			Namespace: metav1.NamespaceSystem,
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     "system:bootstrappers:worker-config-1.35",
		},
		Subjects: []rbacv1.Subject{
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:bootstrappers",
			},
			{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Group",
				Name:     "system:nodes",
			},
		},
	}, metav1.CreateOptions{})
	s.Require().NoError(err)
}

func TestContainerdUpgradeSuite(t *testing.T) {
	s := ContainerdUpgradeSuite{
		common.BootlooseSuite{
			ControllerCount: 1,
			WorkerCount:     1,
		},
	}
	suite.Run(t, &s)
}
