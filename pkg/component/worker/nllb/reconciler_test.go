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
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/net"
	"github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/config"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/avast/retry-go"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"
)

func TestReconciler_Lifecycle(t *testing.T) {
	createdReconciler := func(t *testing.T) *Reconciler {
		t.Helper()
		t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

		apiServer, err := net.NewHostPort("127.99.99.99", 6443)
		require.NoError(t, err)

		dataDir := t.TempDir()
		reconciler, err := NewReconciler(
			&config.CfgVars{
				DataDir:               dataDir,
				KubeletAuthConfigPath: writeKubeconfig(t),
			},
			nil,
			t.Name(),
			workerconfig.Profile{
				APIServerAddresses: []net.HostPort{*apiServer},
				NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
					Enabled:    true,
					Type:       v1beta1.NllbTypeEnvoyProxy,
					EnvoyProxy: v1beta1.DefaultEnvoyProxy(),
				},
				Konnectivity: workerconfig.Konnectivity{
					AgentPort: 1337,
				},
			},
		)
		require.NoError(t, err)
		reconciler.log = newTestLogger(t)
		reconciler.loadBalancer = new(backendMock)

		return reconciler
	}

	t.Run("when_created", func(t *testing.T) {

		t.Run("init_succeeds", func(t *testing.T) {
			underTest := createdReconciler(t)
			ctx := testContext(t)
			loadBalancer := underTest.loadBalancer.(*backendMock)
			loadBalancer.On("init", ctx).Return(nil)

			err := underTest.Init(ctx)

			assert.NoError(t, err)
			loadBalancer.AssertExpectations(t)
		})

		t.Run("start_fails", func(t *testing.T) {
			underTest := createdReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: created", err.Error())
		})

		t.Run("ready_fails", func(t *testing.T) {
			underTest := createdReconciler(t)

			err := underTest.Ready()

			require.Error(t, err)
			assert.Equal(t, "cannot check for readiness, not started: created", err.Error())
		})

		t.Run("stop_fails", func(t *testing.T) {
			underTest := createdReconciler(t)

			err := underTest.Stop()

			require.Error(t, err)
			assert.Equal(t, "cannot stop: created", err.Error())
		})
	})

	initializedReconciler := func(t *testing.T) *Reconciler {
		t.Helper()
		underTest := createdReconciler(t)
		ctx := testContext(t)
		loadBalancer := underTest.loadBalancer.(*backendMock)
		loadBalancer.On("init", ctx).Return(nil)

		require.NoError(t, underTest.Init(ctx))
		return underTest
	}

	t.Run("when_initialized", func(t *testing.T) {

		t.Run("another_init_fails", func(t *testing.T) {
			underTest := initializedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: initialized", err.Error())
		})

		t.Run("start_and_stop_succeed", func(t *testing.T) {
			apiServer, err := net.NewHostPort("127.99.99.99", 6443)
			require.NoError(t, err)
			underTest := initializedReconciler(t)
			ctx := testContext(t)
			loadBalancer := underTest.loadBalancer.(*backendMock)
			loadBalancer.On("start", ctx, mock.Anything, mock.Anything).Return(nil)
			getAPIServerAddressCalled := make(chan struct{})
			loadBalancer.On("getAPIServerAddress").Return(apiServer, nil).Run(func(mock.Arguments) {
				close(getAPIServerAddressCalled)
			})

			require.NoError(t, underTest.Start(ctx))

			select {
			case <-getAPIServerAddressCalled:
				break
			case <-time.After(10 * time.Second):
				require.Fail(t, "Timed out while waiting for call to getAPIServerAddress")
			}
			loadBalancer.AssertExpectations(t)

			loadBalancer.On("stop").Return(nil)

			assert.NoError(t, underTest.Stop())

			loadBalancer.AssertExpectations(t)
		})

		t.Run("ready_fails", func(t *testing.T) {
			underTest := initializedReconciler(t)

			err := underTest.Ready()

			require.Error(t, err)
			assert.Equal(t, "cannot check for readiness, not started: initialized", err.Error())
		})

		t.Run("stop_fails", func(t *testing.T) {
			underTest := initializedReconciler(t)

			err := underTest.Stop()

			require.Error(t, err)
			assert.Equal(t, "cannot stop: initialized", err.Error())
		})
	})

	startedReconciler := func(t *testing.T) (_ *Reconciler, allowStop func()) {
		t.Helper()
		underTest := initializedReconciler(t)
		apiServer, err := net.NewHostPort("127.99.99.99", 6443)
		require.NoError(t, err)
		ctx := testContext(t)
		loadBalancer := underTest.loadBalancer.(*backendMock)
		loadBalancer.On("start", ctx, mock.Anything, mock.Anything).Return(nil)
		loadBalancer.On("getAPIServerAddress").Maybe().Return(apiServer, nil)
		require.NoError(t, underTest.Start(ctx))

		var once sync.Once
		allowStop = func() {
			once.Do(func() {
				loadBalancer.On("stop").Return(nil)
			})
		}

		t.Cleanup(func() {
			allowStop()
			err := underTest.Stop()
			if !t.Failed() {
				assert.NoError(t, err)
				loadBalancer.AssertExpectations(t)
			}
		})

		return underTest, allowStop
	}

	t.Run("when_started", func(t *testing.T) {

		t.Run("init_fails", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: started", err.Error())
		})

		t.Run("another_start_fails", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: started", err.Error())
		})

		t.Run("ready_succeeds", func(t *testing.T) {
			underTest, _ := startedReconciler(t)

			err := underTest.Ready()

			assert.NoError(t, err)
		})

		t.Run("stop_succeeds", func(t *testing.T) {
			underTest, allowStop := startedReconciler(t)
			allowStop()

			err := underTest.Stop()

			assert.NoError(t, err)
		})
	})

	stoppedReconciler := func(t *testing.T) *Reconciler {
		t.Helper()
		underTest, allowStop := startedReconciler(t)
		allowStop()
		require.NoError(t, underTest.Stop())
		return underTest
	}

	t.Run("when_stopped", func(t *testing.T) {

		t.Run("init_fails", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Init(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot initialize, not created: stopped", err.Error())
		})

		t.Run("start_fails", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Start(testContext(t))

			require.Error(t, err)
			assert.Equal(t, "cannot start, not initialized: stopped", err.Error())
		})

		t.Run("stop_succeeds", func(t *testing.T) {
			underTest := stoppedReconciler(t)

			err := underTest.Stop()

			assert.NoError(t, err)
		})
	})
}

func TestReconciler_ConfigMgmt(t *testing.T) {
	newTestInstance := func(t *testing.T, dataDir string, apiServers ...net.HostPort) (*Reconciler, *staticPodMock) {
		staticPod := new(staticPodMock)
		staticPod.On("Drop").Return()

		staticPods := new(staticPodsMock)
		staticPods.On("ClaimStaticPod", mock.Anything, mock.Anything).Return(staticPod, nil)
		reconciler, err := NewReconciler(
			&config.CfgVars{
				DataDir:               dataDir,
				KubeletAuthConfigPath: writeKubeconfig(t),
			},
			staticPods,
			t.Name(),
			workerconfig.Profile{
				APIServerAddresses: apiServers,
				NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
					Enabled:    true,
					Type:       v1beta1.NllbTypeEnvoyProxy,
					EnvoyProxy: v1beta1.DefaultEnvoyProxy(),
				},
				Konnectivity: workerconfig.Konnectivity{
					AgentPort: 1337,
				},
			},
		)
		require.NoError(t, err)
		reconciler.log = newTestLogger(t)
		return reconciler, staticPod
	}

	t.Run("configDir", func(t *testing.T) {
		for _, test := range []struct {
			name    string
			prepare func(t *testing.T, dir string)
		}{
			{"create", func(t *testing.T, dir string) {}},
			{"chmod", func(t *testing.T, dir string) {
				require.NoError(t, os.Mkdir(filepath.Join(dir, "k0s"), 0777))
				require.NoError(t, os.Mkdir(filepath.Join(dir, "k0s", "nllb"), 0777))
			}},
		} {
			t.Run(test.name, func(t *testing.T) {
				runtimeDir := t.TempDir()
				t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
				expectedMode := 0700
				if runtime.GOOS == "windows" {
					expectedMode = 0777 // On Windows, file mode just mimics the read-only flag
				}

				test.prepare(t, runtimeDir)

				underTest, _ := newTestInstance(t, t.TempDir())
				err := underTest.Init(testContext(t))
				require.NoError(t, err)

				nllbDir := filepath.Join(runtimeDir, "k0s", "nllb")
				stat, err := os.Stat(nllbDir)
				require.NoError(t, err)
				assert.True(t, stat.IsDir())

				assert.Equal(t, expectedMode, int(stat.Mode()&fs.ModePerm))
			})
		}

		t.Run("obstructed", func(t *testing.T) {
			runtimeDir := t.TempDir()
			t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
			k0sDir := filepath.Join(runtimeDir, "k0s")
			require.NoError(t, os.WriteFile(k0sDir, []byte("obstructed"), 0777))

			underTest, _ := newTestInstance(t, t.TempDir())
			err := underTest.Init(testContext(t))

			var pathErr *os.PathError
			if assert.ErrorAs(t, err, &pathErr) {
				assert.ErrorIs(t, pathErr.Err, syscall.ENOTDIR)
				assert.Equal(t, k0sDir, pathErr.Path)
			}
		})
	})

	t.Run("configFile", func(t *testing.T) {
		// given
		runtimeDir := t.TempDir()
		t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
		dataDir := t.TempDir()
		envoyConfig := filepath.Join(runtimeDir, "k0s", "nllb", "envoy", "envoy.yaml")
		apiServerAddress, err := net.NewHostPort("127.10.10.1", 6443)
		require.NoError(t, err)

		// when
		underTest, staticPod := newTestInstance(t, dataDir, *apiServerAddress)
		t.Cleanup(func() {
			assert.NoError(t, underTest.Stop())
			assert.NoFileExists(t, envoyConfig)
		})
		err = underTest.Init(testContext(t))
		require.NoError(t, err)

		staticPod.On("SetManifest", mock.AnythingOfType("v1.Pod")).Return(nil)
		err = underTest.Start(testContext(t))
		require.NoError(t, err)

		// then
		assert.NoError(t,
			retry.Do(func() error {
				stat, err := os.Stat(envoyConfig)
				if os.IsNotExist(err) {
					return err
				}

				if err == nil && stat.IsDir() {
					err = errors.New("expected a file")
				}

				if err != nil {
					return retry.Unrecoverable(err)
				}

				return nil
			}, retry.LastErrorOnly(true)),
			"Expected to see an Envoy configuration file",
		)

		configBytes, err := os.ReadFile(envoyConfig)
		if assert.NoError(t, err) {
			var yamlConfig any
			assert.NoError(t, yaml.Unmarshal(configBytes, &yamlConfig), "invalid YAML in config file: %s", string(configBytes))
		}
	})
}

func TestReconciler_APIServerAddressFromKubeconfig(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())

	dataDir := t.TempDir()
	apiServer, err := net.NewHostPort("127.99.99.99", 443)
	require.NoError(t, err)

	loadBalancer := new(backendMock)
	loadBalancer.On("init", mock.Anything).Return(nil)
	loadBalancer.On("start", mock.Anything, mock.Anything, []net.HostPort{*apiServer}).Return(nil)
	loadBalancer.On("getAPIServerAddress").Maybe().Return(apiServer, nil)
	loadBalancer.On("stop", mock.Anything).Return(nil)
	underTest, err := NewReconciler(
		&config.CfgVars{
			DataDir:               dataDir,
			KubeletAuthConfigPath: writeKubeconfig(t),
		},
		nil,
		t.Name(),
		workerconfig.Profile{
			NodeLocalLoadBalancing: &v1beta1.NodeLocalLoadBalancing{
				Enabled:    true,
				Type:       v1beta1.NllbTypeEnvoyProxy,
				EnvoyProxy: v1beta1.DefaultEnvoyProxy(),
			},
			Konnectivity: workerconfig.Konnectivity{
				AgentPort: 1337,
			},
		},
	)
	require.NoError(t, err)
	underTest.log = newTestLogger(t)
	underTest.loadBalancer = loadBalancer
	require.NoError(t, underTest.Init(testContext(t)))
	require.NoError(t, underTest.Start(testContext(t)))
	assert.NoError(t, underTest.Stop())

	loadBalancer.AssertExpectations(t)
}

func writeKubeconfig(t *testing.T) string {
	t.Helper()

	const fake = "fake"
	kubeconfig := clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{fake: {Server: "https://127.99.99.99/fake"}},
		Contexts:       map[string]*clientcmdapi.Context{fake: {Cluster: fake}},
		CurrentContext: fake,
	}

	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, clientcmd.WriteToFile(kubeconfig, path))
	return path
}

func testContext(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.TODO())
	timeout := time.AfterFunc(10*time.Second, func() {
		assert.Fail(t, "Test context timed out after 10 seconds")
		cancel()
	})
	t.Cleanup(func() {
		timeout.Stop()
		cancel()
	})
	return ctx
}

func newTestLogger(t *testing.T) logrus.FieldLogger {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return log.WithField("test", t.Name())
}

type backendMock struct{ mock.Mock }

func (m *backendMock) getAPIServerAddress() (*net.HostPort, error) {
	args := m.Called()
	return args.Get(0).(*net.HostPort), args.Error(1)
}

func (m *backendMock) init(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *backendMock) start(ctx context.Context, profile workerconfig.Profile, apiServers []net.HostPort) error {
	args := m.Called(ctx, profile, apiServers)
	return args.Error(0)
}

func (m *backendMock) stop() {
	m.Called()
}

func (m *backendMock) updateAPIServers(apiServers []net.HostPort) error {
	args := m.Called(apiServers)
	return args.Error(0)
}

type staticPodsMock struct{ mock.Mock }

func (m *staticPodsMock) ManifestURL() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *staticPodsMock) ClaimStaticPod(namespace, name string) (worker.StaticPod, error) {
	args := m.Called(namespace, name)
	return args.Get(0).(worker.StaticPod), args.Error(1)
}

type staticPodMock struct{ mock.Mock }

func (m *staticPodMock) SetManifest(podResource interface{}) error {
	args := m.Called(podResource)
	return args.Error(0)
}

func (m *staticPodMock) Clear() {
	m.Called()
}

func (m *staticPodMock) Drop() {
	m.Called()
}
