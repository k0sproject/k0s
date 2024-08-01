/*
Copyright 2022 k0s authors

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

package nllb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"text/template"

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
)

// envoyProxy is a load balancer [backend] that's managing a static Envoy pod to
// implement node-local load balancing.
type envoyProxy struct {
	log logrus.FieldLogger

	dir        string
	staticPods worker.StaticPods

	pod    worker.StaticPod
	config *envoyConfig
}

var _ backend = (*envoyProxy)(nil)

// envoyParams holds common parameters that are shared between all reconcilable parts.
type envoyParams struct {
	// Directory in which the envoy config files are stored.
	configDir string

	// IP to which Envoy will bind.
	bindIP net.IP

	// Port to which Envoy will bind the API server load balancer.
	apiServerBindPort uint16

	// Port to which Konnectivity will bind.
	konnectivityServerBindPort uint16
}

// envoyPodParams holds the parameters for the static Envoy pod template.
type envoyPodParams struct {
	// The Envoy image to pull.
	image v1beta1.ImageSpec

	// The pull policy to use for the Envoy container.
	pullPolicy corev1.PullPolicy
}

// envoyFilesParams holds the parameters for the Envoy config files.
type envoyFilesParams struct {
	// Addresses on which the upstream API servers are listening.
	apiServers []k0snet.HostPort

	// Port on which the upstream konnectivity servers are listening.
	konnectivityServerPort uint16
}

// envoyConfig is a convenience struct that combines all envoy parameters.
type envoyConfig struct {
	envoyParams
	envoyPodParams
	envoyFilesParams
}

const (
	envoyBootstrapFile = "envoy.yaml"
	envoyCDSFile       = "cds.yaml"
)

func (e *envoyProxy) init(ctx context.Context) error {
	if err := dir.Init(e.dir, 0755); err != nil {
		return err
	}

	return nil
}

func (e *envoyProxy) start(ctx context.Context, profile workerconfig.Profile, apiServers []k0snet.HostPort) (err error) {
	if e.config != nil {
		return errors.New("already started")
	}

	e.pod, err = e.staticPods.ClaimStaticPod("kube-system", "nllb")
	if err != nil {
		e.pod = nil
		return fmt.Errorf("failed to claim static pod for EnvoyProxy: %w", err)
	}

	defer func() {
		if err != nil {
			pod := e.pod
			pod.Clear()
			e.pod = nil
			e.stop()
			e.pod = pod
		}
	}()

	loopbackIP, err := getLoopbackIP(ctx)
	if err != nil {
		if errors.Is(err, ctx.Err()) {
			return err
		}
		e.log.WithError(err).Infof("Falling back to %s as bind address", loopbackIP)
	}

	nllb := profile.NodeLocalLoadBalancing
	var konnectivityBindPort uint16
	if nllb.EnvoyProxy.KonnectivityServerBindPort != nil {
		konnectivityBindPort = uint16(*nllb.EnvoyProxy.KonnectivityServerBindPort)
	}

	e.config = &envoyConfig{
		envoyParams{
			e.dir,
			loopbackIP,
			uint16(profile.NodeLocalLoadBalancing.EnvoyProxy.APIServerBindPort),
			konnectivityBindPort,
		},
		envoyPodParams{
			*nllb.EnvoyProxy.Image,
			nllb.EnvoyProxy.ImagePullPolicy,
		},
		envoyFilesParams{
			konnectivityServerPort: profile.Konnectivity.AgentPort,
			apiServers:             apiServers,
		},
	}

	err = writeEnvoyConfigFiles(&e.config.envoyParams, &e.config.envoyFilesParams)
	if err != nil {
		return err
	}

	err = e.provision()
	if err != nil {
		return err
	}

	return nil
}

func (e *envoyProxy) getAPIServerAddress() (*k0snet.HostPort, error) {
	if e.config == nil {
		return nil, errors.New("not yet started")
	}
	return k0snet.NewHostPort(e.config.bindIP.String(), e.config.apiServerBindPort)
}

func (e *envoyProxy) updateAPIServers(apiServers []k0snet.HostPort) error {
	if e.config == nil {
		return errors.New("not yet started")
	}
	e.config.envoyFilesParams.apiServers = apiServers
	return writeEnvoyConfigFiles(&e.config.envoyParams, &e.config.envoyFilesParams)
}

func (e *envoyProxy) stop() {
	if e.pod != nil {
		e.pod.Drop()
		e.pod = nil
	}

	if e.config == nil {
		return
	}

	for _, file := range []struct{ desc, name string }{
		{"Envoy bootstrap config", envoyBootstrapFile},
		{"Envoy CDS config", envoyCDSFile},
	} {
		path := filepath.Join(e.config.configDir, file.name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			e.log.WithError(err).Warnf("Failed to remove %s from disk", file.desc)
		}
	}

	e.config = nil
}

func writeEnvoyConfigFiles(params *envoyParams, filesParams *envoyFilesParams) error {
	data := struct {
		BindIP                     net.IP
		APIServerBindPort          uint16
		KonnectivityServerBindPort uint16
		KonnectivityServerPort     uint16
		UpstreamServers            []k0snet.HostPort
	}{
		BindIP:                     params.bindIP,
		APIServerBindPort:          params.apiServerBindPort,
		KonnectivityServerBindPort: params.konnectivityServerBindPort,
		KonnectivityServerPort:     filesParams.konnectivityServerPort,
		UpstreamServers:            filesParams.apiServers,
	}

	var errs []error
	for fileName, template := range map[string]*template.Template{
		envoyBootstrapFile: envoyBootstrapConfig,
		envoyCDSFile:       envoyClustersConfig,
	} {
		err := file.WriteAtomically(filepath.Join(params.configDir, fileName), 0444, func(file io.Writer) error {
			bufferedWriter := bufio.NewWriter(file)
			if err := template.Execute(bufferedWriter, data); err != nil {
				return fmt.Errorf("failed to render template: %w", err)
			}
			return bufferedWriter.Flush()
		})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to write %s: %w", fileName, err))
		}
	}

	return errors.Join(errs...)
}

func (e *envoyProxy) provision() error {
	manifest := makePodManifest(&e.config.envoyParams, &e.config.envoyPodParams)
	if err := e.pod.SetManifest(manifest); err != nil {
		return err
	}

	e.log.Info("Provisioned static Envoy Pod")
	return nil
}

func makePodManifest(params *envoyParams, podParams *envoyPodParams) corev1.Pod {
	ports := []corev1.ContainerPort{
		{Name: "api-server", ContainerPort: int32(params.apiServerBindPort), Protocol: corev1.ProtocolTCP},
	}
	if params.konnectivityServerBindPort != 0 {
		ports = append(ports, corev1.ContainerPort{Name: "konnectivity", ContainerPort: int32(params.konnectivityServerBindPort), Protocol: corev1.ProtocolTCP})
	}
	return corev1.Pod{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nllb",
			Namespace: "kube-system",
			Labels:    applier.CommonLabels("nllb"),
		},
		Spec: corev1.PodSpec{
			HostNetwork: true,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: ptr.To(true),
			},
			Containers: []corev1.Container{{
				Name:            "nllb",
				Image:           podParams.image.URI(),
				ImagePullPolicy: podParams.pullPolicy,
				Ports:           ports,
				Args:            []string{"-c", "/etc/envoy/envoy.yaml", "--use-dynamic-base-id"},
				SecurityContext: &corev1.SecurityContext{
					ReadOnlyRootFilesystem:   ptr.To(true),
					Privileged:               ptr.To(false),
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
				},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "envoy-config",
					MountPath: "/etc/envoy",
					ReadOnly:  true,
				}},
				LivenessProbe: &corev1.Probe{
					PeriodSeconds:    10,
					FailureThreshold: 3,
					TimeoutSeconds:   3,
					ProbeHandler: corev1.ProbeHandler{
						TCPSocket: &corev1.TCPSocketAction{
							Host: params.bindIP.String(), Port: intstr.FromInt(int(params.apiServerBindPort)),
						},
					},
				},
			}},
			Volumes: []corev1.Volume{{
				Name: "envoy-config",
				VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: params.configDir,
						Type: (*corev1.HostPathType)(ptr.To(string(corev1.HostPathDirectory))),
					},
				}},
			},
		},
	}
}

var envoyBootstrapConfig = template.Must(template.New("Bootstrap").Parse(`
node:
  cluster: nllb-cluster
  id: nllb-id

dynamic_resources:
  cds_config:
    path: /etc/envoy/cds.yaml

{{ $localKonnectivityPort := .KonnectivityServerBindPort -}}
{{- $remoteKonnectivityPort := .KonnectivityServerPort -}}
static_resources:
  listeners:
  - name: apiserver
    address:
      socket_address: { address: {{ printf "%q" .BindIP }}, port_value: {{ .APIServerBindPort }} }
    filter_chains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          stat_prefix: apiserver
          cluster: apiserver
  {{- if ne $localKonnectivityPort 0 }}
  - name: konnectivity
    address:
      socket_address: { address: {{ printf "%q" .BindIP }}, port_value: {{ $localKonnectivityPort }} }
    filter_chains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          stat_prefix: konnectivity
          cluster: konnectivity
  {{- end }}
`))

var envoyClustersConfig = template.Must(template.New("Clusters").Parse(`
{{- $localKonnectivityPort := .KonnectivityServerBindPort -}}
{{- $remoteKonnectivityPort := .KonnectivityServerPort -}}
resources:
- "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
  name: apiserver
  connect_timeout: 0.25s
  type: STATIC
  lb_policy: RANDOM
  load_assignment:
    cluster_name: apiserver
    endpoints:
    - lb_endpoints:
      {{- range .UpstreamServers }}
      - endpoint:
          address:
            socket_address:
              address: {{ printf "%q" .Host }}
              port_value: {{ .Port }}
      {{- else }} []{{ end }}
  health_checks:
  # FIXME: Better use a proper HTTP based health check, but this needs certs and stuff...
  - tcp_health_check: {}
    timeout: 1s
    interval: 5s
    healthy_threshold: 3
    unhealthy_threshold: 5
  {{- if ne $localKonnectivityPort 0 }}
- "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
  name: konnectivity
  connect_timeout: 0.25s
  type: STATIC
  lb_policy: ROUND_ROBIN
  load_assignment:
    cluster_name: konnectivity
    endpoints:
    - lb_endpoints:
      {{- range .UpstreamServers }}
      - endpoint:
          address:
            socket_address:
              address: {{ printf "%q" .Host }}
              port_value: {{ $remoteKonnectivityPort }}
      {{- else }} []{{ end }}
  health_checks:
  # FIXME: What would be a proper health check?
  - tcp_health_check: {}
    timeout: 1s
    interval: 5s
    healthy_threshold: 3
    unhealthy_threshold: 5
{{- end }}
`))
