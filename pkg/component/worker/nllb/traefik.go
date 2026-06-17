// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package nllb

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	k0snet "github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

// TODO: Currently renders konnectivity config even if konnectivity is not
// enabled. Double check with envoy, if it's the case over there, too?

type traefik struct {
	log logrus.FieldLogger

	dir        string
	staticPods worker.StaticPods

	pod    worker.StaticPod
	config *traefikConfig
}

var _ backend = (*traefik)(nil)

type traefikPodParams struct {
	image         v1beta1.ImageSpec
	pullPolicy    corev1.PullPolicy
	hostConfigDir string
}

type traefikInstallConfig struct {
	bindIP                     net.IP
	apiServerBindPort          uint16
	konnectivityServerBindPort uint16
}

type traefikRoutingConfig struct {
	apiServers             []k0snet.HostPort
	konnectivityServerPort uint16
}

type traefikConfig struct {
	traefikPodParams
	traefikInstallConfig
	traefikRoutingConfig
}

const (
	traefikInstallConfigFileName = "config.yaml"
	traefikRoutingConfigFileName = "routing.yaml"
)

func (t *traefik) init(ctx context.Context) error {
	if err := dir.InitWithOptions(t.dir).WithPermissions(0755).WithSELinuxLabel(containerFileLabel).Apply(); err != nil {
		return err
	}

	return nil
}

func (t *traefik) start(ctx context.Context, profile workerconfig.Profile, apiServers []k0snet.HostPort) (err error) {
	if t.config != nil {
		return errors.New("already started")
	}

	t.pod, err = t.staticPods.ClaimStaticPod(metav1.NamespaceSystem, "nllb")
	if err != nil {
		t.pod = nil
		return fmt.Errorf("failed to claim static pod for Traefik: %w", err)
	}

	defer func() {
		if err != nil {
			pod := t.pod
			pod.Clear()
			t.pod = nil
			t.stop()
			t.pod = pod
		}
	}()

	loopbackIP, err := getLoopbackIP(ctx)
	if err != nil {
		if errors.Is(err, ctx.Err()) {
			return err
		}
		t.log.WithError(err).Infof("Falling back to %s as bind address", loopbackIP)
	}

	nllb := profile.NodeLocalLoadBalancing
	var konnectivityBindPort uint16
	if nllb.Traefik.KonnectivityServerBindPort != nil {
		konnectivityBindPort = uint16(*nllb.Traefik.KonnectivityServerBindPort)
	}

	t.config = &traefikConfig{
		traefikPodParams: traefikPodParams{
			image:         *nllb.Traefik.Image,
			pullPolicy:    nllb.Traefik.ImagePullPolicy,
			hostConfigDir: t.dir,
		},
		traefikInstallConfig: traefikInstallConfig{
			bindIP:                     loopbackIP,
			apiServerBindPort:          uint16(nllb.Traefik.APIServerBindPort),
			konnectivityServerBindPort: konnectivityBindPort,
		},
		traefikRoutingConfig: traefikRoutingConfig{
			apiServers:             apiServers,
			konnectivityServerPort: profile.Konnectivity.AgentPort,
		},
	}

	if err := t.writeInstallConfigFile(); err != nil {
		return err
	}
	if err := t.writeRoutingConfigFile(); err != nil {
		return err
	}

	return t.provision()
}

func (t *traefik) getAPIServerAddress() (*k0snet.HostPort, error) {
	if t.config == nil {
		return nil, errors.New("not yet started")
	}
	return k0snet.NewHostPort(t.config.bindIP.String(), t.config.apiServerBindPort)
}

func (t *traefik) updateAPIServers(apiServers []k0snet.HostPort) error {
	if t.config == nil {
		return errors.New("not yet started")
	}
	t.config.apiServers = apiServers
	return t.writeRoutingConfigFile()
}

func (t *traefik) writeInstallConfigFile() error {
	content, err := t.config.traefikInstallConfig.toFileContent()
	if err != nil {
		return fmt.Errorf("failed to generate install configuration: %w", err)
	}
	return t.writeConfigFile(traefikInstallConfigFileName, content)
}

func (t *traefik) writeRoutingConfigFile() error {
	content, err := t.config.traefikRoutingConfig.toFileContent(&t.config.traefikInstallConfig)
	if err != nil {
		return fmt.Errorf("failed to generate routing configuration: %w", err)
	}
	return t.writeConfigFile(traefikRoutingConfigFileName, content)
}

func (t *traefik) stop() {
	if t.pod != nil {
		t.pod.Drop()
		t.pod = nil
	}

	if t.config == nil {
		return
	}

	for _, file := range []struct{ desc, name string }{
		{"Traefik install configuration", traefikInstallConfigFileName},
		{"Traefik routing configuration", traefikRoutingConfigFileName},
	} {
		path := filepath.Join(t.dir, file.name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			t.log.WithError(err).Warnf("Failed to remove %s from disk", file.desc)
		}
	}

	t.config = nil
}

func (t *traefik) writeConfigFile(name string, content []byte) error {
	return file.AtomicWithTarget(filepath.Join(t.dir, name)).
		WithPermissions(0444).
		WithSELinuxLabel(containerFileLabel).
		Write(content)
}

func (t *traefik) provision() error {
	manifest := makeTraefikPodManifest(&t.config.traefikPodParams, &t.config.traefikInstallConfig)
	if err := t.pod.SetManifest(manifest); err != nil {
		return err
	}

	t.log.Info("Provisioned static Traefik Pod")
	return nil
}

func traefikContainerConfigDir() string {
	return filepath.Join(string(filepath.Separator), "etc", "traefik")
}

func makeTraefikPodManifest(podParams *traefikPodParams, installConfig *traefikInstallConfig) *corev1.Pod {
	ports := []corev1.ContainerPort{
		{Name: "api-server", ContainerPort: int32(installConfig.apiServerBindPort), Protocol: corev1.ProtocolTCP},
	}
	if installConfig.konnectivityServerBindPort != 0 {
		ports = append(ports, corev1.ContainerPort{Name: "konnectivity", ContainerPort: int32(installConfig.konnectivityServerBindPort), Protocol: corev1.ProtocolTCP})
	}

	configDir := traefikContainerConfigDir()

	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nllb",
			Namespace: metav1.NamespaceSystem,
			Labels:    applier.CommonLabels("nllb"),
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			// The Traefik Pod is the worker's load-balanced path to the control
			// plane, so it must outlive ordinary workloads during graceful node
			// shutdown and be protected from node-pressure eviction.
			//
			// PriorityClassName satisfies the kube-apiserver Priority admission
			// controller, which validates the mirror Pod the kubelet registers
			// for this static Pod. The numeric Priority is also set so the local
			// kubelet (which does not resolve PriorityClassName for static Pods)
			// uses it for shutdown/eviction ordering. The two must agree:
			// admission computes the integer from the class name and rejects the
			// mirror Pod if an explicit, mismatched Priority is provided.
			PriorityClassName: "system-node-critical",
			Priority:          ptr.To(int32(2000001000)),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
				// https://kubernetes.io/docs/tasks/configure-pod-container/create-hostprocess-pod/
				WindowsOptions: &corev1.WindowsSecurityContextOptions{
					HostProcess:   ptr.To(true),
					RunAsUserName: ptr.To(`NT AUTHORITY\Local service`),
				},
			},
			Containers: []corev1.Container{{
				Name:            "nllb",
				Image:           podParams.image.URI(),
				ImagePullPolicy: podParams.pullPolicy,
				Ports:           ports,
				SecurityContext: &corev1.SecurityContext{
					ReadOnlyRootFilesystem:   ptr.To(true),
					Privileged:               ptr.To(false),
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				Args: []string{"--configFile=" + filepath.Join(configDir, traefikInstallConfigFileName)},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "traefik-config",
					MountPath: configDir,
					ReadOnly:  true,
				}},
				LivenessProbe: &corev1.Probe{
					PeriodSeconds:    10,
					FailureThreshold: 3,
					TimeoutSeconds:   3,
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{
							Host: installConfig.bindIP.String(), Port: intstr.FromInt(int(installConfig.apiServerBindPort)),
						},
					},
				},
			}},
			Volumes: []corev1.Volume{{
				Name: "traefik-config",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: podParams.hostConfigDir,
						Type: ptr.To(corev1.HostPathDirectory),
					},
				},
			}},
			Tolerations: []corev1.Toleration{{
				Operator: corev1.TolerationOpExists,
			}},
			EnableServiceLinks: ptr.To(false),
		},
	}
}

func (i *traefikInstallConfig) toFileContent() ([]byte, error) {
	bindIP := i.bindIP.String()
	entryPoints := map[string]any{
		"apiserver": map[string]any{
			"address": net.JoinHostPort(bindIP, strconv.FormatUint(uint64(i.apiServerBindPort), 10)),
		},
	}
	if i.konnectivityServerBindPort != 0 {
		entryPoints["konnectivity"] = map[string]any{
			"address": net.JoinHostPort(bindIP, strconv.FormatUint(uint64(i.konnectivityServerBindPort), 10)),
		}
	}

	return yaml.Marshal(map[string]any{
		"global": map[string]any{
			"checkNewVersion":    false,
			"sendAnonymousUsage": false,
		},
		"log": map[string]any{
			"level":   "INFO",
			"noColor": true,
		},
		"entryPoints": entryPoints,
		"providers": map[string]any{
			"file": map[string]any{
				"filename": filepath.Join(traefikContainerConfigDir(), traefikRoutingConfigFileName),
				"watch":    true,
			},
		},
	})
}

func (r *traefikRoutingConfig) toFileContent(installConfig *traefikInstallConfig) ([]byte, error) {
	const (
		apiserver    = "apiserver"
		konnectivity = "konnectivity"
	)

	routers := make(map[string]any)
	services := make(map[string]any)

	servers := make([]map[string]any, len(r.apiServers))
	for i := range r.apiServers {
		servers[i] = map[string]any{
			"address": r.apiServers[i].String(),
		}
	}

	routers[apiserver] = map[string]any{
		"entryPoints": []string{apiserver},
		"rule":        "HostSNI(`*`)",
		"service":     apiserver,
	}
	services[apiserver] = map[string]any{
		"loadBalancer": map[string]any{
			"servers": servers,
		},
	}

	if installConfig.konnectivityServerBindPort != 0 {
		port := strconv.FormatUint(uint64(r.konnectivityServerPort), 10)
		servers := make([]map[string]any, len(r.apiServers))
		for i := range r.apiServers {
			servers[i] = map[string]any{
				"address": net.JoinHostPort(r.apiServers[i].Host(), port),
			}
		}

		routers[konnectivity] = map[string]any{
			"entryPoints": []string{konnectivity},
			"rule":        "HostSNI(`*`)",
			"service":     konnectivity,
		}
		services[konnectivity] = map[string]any{
			"loadBalancer": map[string]any{
				"servers": servers,
			},
		}
	}

	return yaml.Marshal(map[string]any{
		"tcp": map[string]any{
			"routers":  routers,
			"services": services,
		},
	})
}
