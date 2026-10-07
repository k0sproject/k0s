// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package externalnodes

import (
	"testing"

	"github.com/k0sproject/k0s/pkg/applier"
	"github.com/k0sproject/k0s/pkg/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kubeProxySources(t *testing.T, server string) (*corev1.ConfigMap, *appsv1.DaemonSet) {
	t.Helper()
	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{Clusters: map[string]*clientcmdapi.Cluster{"default": {Server: server}}})
	require.NoError(t, err)
	stackLabels := map[string]string{applier.NameLabel: "kubeproxy", appLabel: kubeProxyName}
	stackAnnotations := map[string]string{applier.ChecksumAnnotation: "abc"}
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: kubeProxyName, Labels: stackLabels, Annotations: stackAnnotations},
		Data:       map[string]string{kubeProxyConfKey: string(kubeconfig), "config.conf": "config"},
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: kubeProxyName, Labels: stackLabels, Annotations: stackAnnotations},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{appLabel: kubeProxyName}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{appLabel: kubeProxyName}},
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{"kubernetes.io/os": "linux"},
					Containers:   []corev1.Container{{Name: kubeProxyName}},
					Volumes: []corev1.Volume{{Name: kubeProxyName, VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: kubeProxyName}},
					}}},
				},
			},
		},
	}
	return cm, ds
}

func konnectivitySource() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: konnectivityName, Labels: map[string]string{applier.NameLabel: "konnectivity"}},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{appLabel: konnectivityName}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{appLabel: konnectivityName}},
				Spec: corev1.PodSpec{
					HostNetwork:     true,
					SecurityContext: &corev1.PodSecurityContext{WindowsOptions: &corev1.WindowsSecurityContextOptions{HostProcess: new(true)}},
					Containers: []corev1.Container{{Name: konnectivityName, Args: []string{
						"--logtostderr=true", "--proxy-server-host=localhost", "--proxy-server-port=7132",
					}}},
				},
			},
		},
	}
}

func assertExternalNodesOnly(t *testing.T, pod *corev1.PodSpec) {
	t.Helper()
	require.NotNil(t, pod.Affinity)
	terms := pod.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	require.Len(t, terms, 1)
	assert.Equal(t, []corev1.NodeSelectorRequirement{{Key: constant.K0sNodeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"false"}}}, terms[0].MatchExpressions)
}

func TestKubeProxyCopy(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	// The original keeps off nodes without k0s, as it does with node-local load balancing.
	ds.Spec.Template.Spec.Affinity = requireNode(nil, corev1.NodeSelectorOpNotIn)
	original := ds.Spec.Template.Spec.Affinity.DeepCopy()
	cmCopy, dsCopy, err := kubeProxyCopy(cm, ds, "https://10.0.0.1:6443")
	require.NoError(t, err)

	assert.Equal(t, "kube-proxy-external", cmCopy.Name)
	assert.NotContains(t, cmCopy.Labels, applier.NameLabel, "the copy must not be pruned with the original stack")
	assert.NotContains(t, cmCopy.Annotations, applier.ChecksumAnnotation)
	kubeconfig, err := clientcmd.Load([]byte(cmCopy.Data[kubeProxyConfKey]))
	require.NoError(t, err)
	assert.Equal(t, "https://10.0.0.1:6443", kubeconfig.Clusters["default"].Server)
	assert.Equal(t, "config", cmCopy.Data["config.conf"])
	assert.Contains(t, cm.Data[kubeProxyConfKey], "localhost", "the original is not modified")

	assert.Equal(t, "kube-proxy-external", dsCopy.Name)
	assert.Equal(t, map[string]string{appLabel: "kube-proxy-external"}, dsCopy.Spec.Selector.MatchLabels)
	assert.Equal(t, "kube-proxy-external", dsCopy.Spec.Template.Labels[appLabel])
	assert.Equal(t, "kube-proxy-external", dsCopy.Spec.Template.Spec.Volumes[0].ConfigMap.Name)
	assert.Equal(t, map[string]string{"kubernetes.io/os": "linux"}, dsCopy.Spec.Template.Spec.NodeSelector)
	assertExternalNodesOnly(t, &dsCopy.Spec.Template.Spec)
	assert.Equal(t, original, ds.Spec.Template.Spec.Affinity, "the original is not modified")
}

func TestKonnectivityCopy(t *testing.T) {
	ds := konnectivitySource()
	dsCopy := konnectivityCopy(ds, "10.0.0.1", 8132)

	assert.Equal(t, "konnectivity-agent-external", dsCopy.Name)
	pod := &dsCopy.Spec.Template.Spec
	assert.False(t, pod.HostNetwork)
	assert.Nil(t, pod.SecurityContext.WindowsOptions)
	assert.Equal(t, []string{"--logtostderr=true", "--proxy-server-host=10.0.0.1", "--proxy-server-port=8132"}, pod.Containers[0].Args)
	assertExternalNodesOnly(t, pod)
	assert.True(t, ds.Spec.Template.Spec.HostNetwork, "the original is not modified")
	assert.Equal(t, "--proxy-server-host=localhost", ds.Spec.Template.Spec.Containers[0].Args[1])
}

func TestKubeProxyCopy_AllClusters(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")
	kubeconfig, err := clientcmd.Write(clientcmdapi.Config{Clusters: map[string]*clientcmdapi.Cluster{
		"default": {Server: "https://localhost:7443"},
		"other":   {Server: "https://localhost:7443"},
	}})
	require.NoError(t, err)
	cm.Data[kubeProxyConfKey] = string(kubeconfig)
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, corev1.Volume{Name: "other", VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: "other"}},
	}})

	cmCopy, dsCopy, err := kubeProxyCopy(cm, ds, "https://[fd00::1]:6443")
	require.NoError(t, err)
	copied, err := clientcmd.Load([]byte(cmCopy.Data[kubeProxyConfKey]))
	require.NoError(t, err)
	for name, cluster := range copied.Clusters {
		assert.Equal(t, "https://[fd00::1]:6443", cluster.Server, name)
	}
	assert.Equal(t, "other", dsCopy.Spec.Template.Spec.Volumes[1].ConfigMap.Name, "other volumes are kept as is")
}

func TestKonnectivityCopy_Containers(t *testing.T) {
	ds := konnectivitySource()
	ds.Spec.Template.Spec.SecurityContext = nil
	ds.Spec.Template.Spec.Containers = append(ds.Spec.Template.Spec.Containers, corev1.Container{
		Name: "sidecar", Args: []string{"--proxy-server-host=localhost", "--keep=me"},
	})

	pod := &konnectivityCopy(ds, "fd00::1", 8132).Spec.Template.Spec
	assert.Nil(t, pod.SecurityContext)
	assert.Equal(t, []string{"--logtostderr=true", "--proxy-server-host=fd00::1", "--proxy-server-port=8132"}, pod.Containers[0].Args)
	assert.Equal(t, []string{"--proxy-server-host=fd00::1", "--keep=me"}, pod.Containers[1].Args)
}

func TestRequireNode(t *testing.T) {
	other := corev1.NodeSelectorRequirement{Key: "kubernetes.io/os", Operator: corev1.NodeSelectorOpIn, Values: []string{"linux"}}
	affinity := &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{MatchExpressions: []corev1.NodeSelectorRequirement{other}},
			{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: constant.K0sNodeLabel, Operator: corev1.NodeSelectorOpNotIn, Values: []string{"false"}}}},
		}},
	}}

	// Each term gets the requirement once, other requirements are kept, and applying it again changes nothing.
	want := []corev1.NodeSelectorTerm{
		{MatchExpressions: []corev1.NodeSelectorRequirement{other, {Key: constant.K0sNodeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"false"}}}},
		{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: constant.K0sNodeLabel, Operator: corev1.NodeSelectorOpIn, Values: []string{"false"}}}},
	}
	affinity = requireNode(affinity, corev1.NodeSelectorOpIn)
	assert.Equal(t, want, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)
	affinity = requireNode(affinity, corev1.NodeSelectorOpIn)
	assert.Equal(t, want, affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms)

	// Preferences and pod affinities stay as they are.
	preferred := []corev1.PreferredSchedulingTerm{{Weight: 1}}
	affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution = preferred
	affinity.PodAntiAffinity = &corev1.PodAntiAffinity{}
	affinity = requireNode(affinity, corev1.NodeSelectorOpIn)
	assert.Equal(t, preferred, affinity.NodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution)
	assert.NotNil(t, affinity.PodAntiAffinity)
}

func TestCopyMeta(t *testing.T) {
	meta := &metav1.ObjectMeta{
		Namespace: "kube-system", Name: "kube-proxy", ResourceVersion: "42", UID: "uid",
		Labels: map[string]string{applier.NameLabel: "kubeproxy", appLabel: "kube-proxy", "keep": "label"},
		Annotations: map[string]string{
			applier.ChecksumAnnotation: "abc", applier.LastConfigAnnotation: "{}",
			"deprecated.daemonset.template.generation": "3", "keep": "annotation",
		},
	}
	assert.Equal(t, metav1.ObjectMeta{
		Namespace: "kube-system", Name: "kube-proxy-external",
		Labels:      map[string]string{appLabel: "kube-proxy", "keep": "label"},
		Annotations: map[string]string{"keep": "annotation"},
	}, copyMeta(meta, "kube-proxy-external"), "server-side and stack metadata are dropped")
	assert.Len(t, meta.Labels, 3, "the original is not modified")
}

func TestKubeProxyCopy_InvalidKubeconfig(t *testing.T) {
	cm, ds := kubeProxySources(t, "https://localhost:7443")

	cm.Data[kubeProxyConfKey] = "invalid"
	_, _, err := kubeProxyCopy(cm, ds, "https://10.0.0.1:6443")
	assert.ErrorContains(t, err, "failed to load the kube-proxy kubeconfig")

	cm.Data[kubeProxyConfKey] = "apiVersion: v1\nkind: Config\n"
	_, _, err = kubeProxyCopy(cm, ds, "https://10.0.0.1:6443")
	assert.ErrorContains(t, err, "has no clusters")
}

func TestDaemonSetCopy_WithoutLabels(t *testing.T) {
	ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "agent"}}
	dsCopy := daemonSetCopy(ds, "agent-external")
	assert.Equal(t, map[string]string{appLabel: "agent-external"}, dsCopy.Labels)
	assert.Equal(t, map[string]string{appLabel: "agent-external"}, dsCopy.Spec.Template.Labels)
	assert.Nil(t, ds.Labels, "the original is not modified")
}
