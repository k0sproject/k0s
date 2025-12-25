// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/internal/pkg/flags"
	"github.com/k0sproject/k0s/internal/pkg/stringmap"
	"github.com/k0sproject/k0s/internal/pkg/sysinfo"
	"github.com/k0sproject/k0s/internal/supervised"
	"github.com/k0sproject/k0s/pkg/build"
	"github.com/k0sproject/k0s/pkg/component/iptables"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/k0sproject/k0s/pkg/component/prober"
	"github.com/k0sproject/k0s/pkg/component/status"
	"github.com/k0sproject/k0s/pkg/component/worker"
	workerconfig "github.com/k0sproject/k0s/pkg/component/worker/config"
	"github.com/k0sproject/k0s/pkg/component/worker/containerd"
	"github.com/k0sproject/k0s/pkg/component/worker/nllb"
	"github.com/k0sproject/k0s/pkg/config"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/kubernetes"
	"github.com/k0sproject/k0s/pkg/node"
	"github.com/k0sproject/k0s/pkg/token"

	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type Command config.CLIOptions

// Interface between an embedded worker and its embedding controller.
type EmbeddingController interface {
	IsSingleNode() bool
}

func NewWorkerCmd() *cobra.Command {
	var (
		debugFlags            internal.DebugFlags
		ignorePreFlightChecks bool
	)

	cmd := &cobra.Command{
		Use:   "worker [join-token]",
		Short: "Run worker",
		Example: `	Command to add worker node to the master node:
	CLI argument:
	$ k0s worker [token]

	or CLI flag:
	$ k0s worker --token-file [path_to_file]
	Note: Token can be passed either as a CLI argument or as a flag`,
		Args:             cobra.MaximumNArgs(1),
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := config.GetCmdOpts(cmd)
			if err != nil {
				return err
			}

			ctx := cmd.Context()
			if err := initLogging(ctx, opts.K0sVars.DataDir); err != nil {
				return fmt.Errorf("failed to initialize logging: %w", err)
			}

			c := (*Command)(opts)
			if len(args) > 0 {
				c.TokenArg = args[0]
			}

			getBootstrapKubeconfig, err := kubeconfigGetterFromJoinToken(c.TokenFile, c.TokenArg)
			if err != nil {
				return err
			}

			nodeName, kubeletExtraArgs, err := GetNodeName(&c.WorkerOptions)
			if err != nil {
				return fmt.Errorf("failed to determine node name: %w", err)
			}

			if err := (&sysinfo.K0sSysinfoSpec{
				ControllerRoleEnabled: false,
				WorkerRoleEnabled:     true,
				DataDir:               c.K0sVars.DataDir,
			}).RunPreFlightChecks(ignorePreFlightChecks); !ignorePreFlightChecks && err != nil {
				return err
			}

			// Check for legacy CA file (unused on worker-only nodes since 1.33)
			if legacyCAFile := filepath.Join(c.K0sVars.CertRootDir, "ca.crt"); file.Exists(legacyCAFile) {
				// Keep the file to allow interop between 1.32 and 1.33.
				// TODO automatically delete this file in future releases.
				logrus.Infof("The file %s is no longer used and can safely be deleted", legacyCAFile)
			}

			// create directories early with the proper permissions
			if err := dir.Init(c.K0sVars.DataDir, constant.DataDirMode); err != nil {
				return err
			}
			if err := dir.Init(c.K0sVars.RunDir, constant.RunDirMode); err != nil {
				return err
			}
			if err := dir.Init(c.K0sVars.BinDir, constant.BinDirMode); err != nil {
				return err
			}

			return c.Start(ctx, nodeName, kubeletExtraArgs, getBootstrapKubeconfig, nil)
		},
	}

	debugFlags.LongRunning().AddToFlagSet(cmd.PersistentFlags())

	flags := cmd.Flags()
	flags.AddFlagSet(config.GetPersistentFlagSet())
	flags.AddFlagSet(config.GetWorkerFlags())
	flags.BoolVar(&ignorePreFlightChecks, "ignore-pre-flight-checks", false, "continue even if pre-flight checks fail")

	return cmd
}

func GetNodeName(opts *config.WorkerOptions) (apitypes.NodeName, stringmap.StringMap, error) {
	// The node name used during bootstrapping needs to match the node name
	// selected by kubelet. Otherwise, kubelet will have problems interacting
	// with a Node object that doesn't match the name in the certificates.
	// https://kubernetes.io/docs/reference/access-authn-authz/node/

	// Kubelet still has some deprecated support for cloud providers, which may
	// completely bypass the "standard" node name detection as it's done here.
	// K0s only supports external cloud providers, which seems to be a dead code
	// path anyways in kubelet. So it's safe to assume that the following code
	// exactly matches the behavior of kubelet.

	kubeletExtraArgs := flags.Split(opts.KubeletExtraArgs)
	nodeName, err := node.GetNodeName(kubeletExtraArgs["--hostname-override"])
	if err != nil {
		return "", nil, err
	}
	return nodeName, kubeletExtraArgs, nil
}

func kubeconfigGetterFromJoinToken(tokenFile, tokenArg string) (clientcmd.KubeconfigGetter, error) {
	if tokenArg != "" {
		if tokenFile != "" {
			return nil, errors.New("you can only pass one token argument either as a CLI argument 'k0s worker [token]' or as a flag 'k0s worker --token-file [path]'")
		}

		kubeconfig, err := loadKubeconfigFromJoinToken(tokenArg)
		if err != nil {
			return nil, err
		}

		return func() (*clientcmdapi.Config, error) {
			return kubeconfig, nil
		}, nil
	}

	if tokenFile == "" {
		return nil, nil
	}

	return func() (*clientcmdapi.Config, error) {
		return loadKubeconfigFromTokenFile(tokenFile)
	}, nil
}

func loadKubeconfigFromJoinToken(tokenData string) (*clientcmdapi.Config, error) {
	decoded, err := token.DecodeJoinToken(tokenData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode join token: %w", err)
	}

	kubeconfig, err := clientcmd.Load(decoded)
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig from join token: %w", err)
	}

	if tokenType := token.GetTokenType(kubeconfig); tokenType != "kubelet-bootstrap" {
		return nil, fmt.Errorf("wrong token type %s, expected type: kubelet-bootstrap", tokenType)
	}

	return kubeconfig, nil
}

func loadKubeconfigFromTokenFile(path string) (*clientcmdapi.Config, error) {
	var problem string
	tokenBytes, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		problem = "not found"
	} else if err != nil {
		return nil, fmt.Errorf("failed to read token file: %w", err)
	} else if len(tokenBytes) == 0 {
		problem = "is empty"
	}
	if problem != "" {
		return nil, fmt.Errorf("token file %q %s"+
			`: obtain a new token via "k0s token create ..." and store it in the file`+
			` or reinstall this node via "k0s install --force ..." or "k0sctl apply --force ..."`,
			path, problem)
	}

	return loadKubeconfigFromJoinToken(string(tokenBytes))
}

// Start starts the worker components based on the given [config.CLIOptions].
func (c *Command) Start(ctx context.Context, nodeName apitypes.NodeName, kubeletExtraArgs stringmap.StringMap, getBootstrapKubeconfig clientcmd.KubeconfigGetter, controller EmbeddingController) error {
	if err := worker.BootstrapKubeletClientConfig(ctx, c.K0sVars, nodeName, &c.WorkerOptions, getBootstrapKubeconfig); err != nil {
		return fmt.Errorf("failed to bootstrap kubelet client configuration: %w", err)
	}

	kubeletKubeconfigPath := c.K0sVars.KubeletAuthConfigPath
	workerConfig, err := workerconfig.LoadProfile(
		ctx,
		kubernetes.KubeconfigFromFile(kubeletKubeconfigPath),
		c.K0sVars.DataDir,
		c.WorkerProfile,
	)
	if err != nil {
		return err
	}

	componentManager := manager.New(prober.DefaultProber)

	// When upgrading controller+worker nodes in a multi-node cluster with a load balancer, the API
	// server address needs to be overridden to point to the local API server. This is needed so
	// that the kubelet will not connect to an API server that is running a previous version of
	// k0s which would violate the Kubernetes version skew policy.
	if controller != nil {
		directKubeconfigPath, err := worker.CreateDirectKubeletKubeconfig(ctx, c.K0sVars, nodeName)
		if err != nil {
			return fmt.Errorf("failed to create direct kubelet kubeconfig: %w", err)
		}
		kubeletKubeconfigPath = directKubeconfigPath
	}

	var staticPods worker.StaticPods

	if workerConfig.NodeLocalLoadBalancing.IsEnabled() {
		if controller != nil && controller.IsSingleNode() {
			return errors.New("node-local load balancing cannot be used in a single-node cluster")
		}

		sp := worker.NewStaticPods()
		reconciler, err := nllb.NewReconciler(c.K0sVars, sp, c.WorkerProfile, *workerConfig.DeepCopy())
		if err != nil {
			return fmt.Errorf("failed to create node-local load balancer reconciler: %w", err)
		}
		// If this is a worker only node, the kubelet should use the NLLB kubelet kubeconfig path
		// rather than the direct kubelet kubeconfig path in the controller+worker mode.
		if controller == nil {
			kubeletKubeconfigPath = reconciler.GetKubeletKubeconfigPath()
		}
		staticPods = sp

		componentManager.Add(ctx, sp)
		componentManager.Add(ctx, reconciler)
	}

	if c.CriSocket == "" {
		componentManager.Add(ctx, containerd.NewComponent(c.LogLevels.Containerd, c.K0sVars, workerConfig))
		componentManager.Add(ctx, worker.NewOCIBundleReconciler(c.K0sVars))
	}

	if controller == nil && runtime.GOOS == "linux" {
		componentManager.Add(ctx, &iptables.Component{
			IPTablesMode: c.IPTablesMode,
			BinDir:       c.K0sVars.BinDir,
		})
	}
	componentManager.Add(ctx,
		&worker.Kubelet{
			NodeName:            nodeName,
			CRISocket:           c.CriSocket,
			EnableCloudProvider: c.CloudProvider,
			K0sVars:             c.K0sVars,
			StaticPods:          staticPods,
			Kubeconfig:          kubeletKubeconfigPath,
			Configuration:       *workerConfig.KubeletConfiguration.DeepCopy(),
			LogLevel:            c.LogLevels.Kubelet,
			Labels:              c.Labels,
			Taints:              c.Taints,
			ExtraArgs:           kubeletExtraArgs,
			DualStackEnabled:    workerConfig.DualStackEnabled,
		})

	certManager := worker.NewCertificateManager(kubeletKubeconfigPath)

	addPlatformSpecificComponents(ctx, componentManager, c.K0sVars, workerConfig, controller, certManager)

	if controller == nil {
		// if running inside a controller, status component is already running
		componentManager.Add(ctx, &status.Status{
			Prober: prober.DefaultProber,
			StatusInformation: status.K0sStatus{
				Pid:        os.Getpid(),
				Role:       "worker",
				Args:       os.Args,
				Version:    build.Version,
				Workloads:  true,
				SingleNode: false,
				K0sVars:    c.K0sVars,
				// worker does not have cluster config. this is only shown in "k0s status -o json".
				// todo: if it's needed, a worker side config client can be set up and used to load the config
				ClusterConfig: nil,
			},
			CertManager: certManager,
			Socket:      c.K0sVars.StatusSocketPath,
		})
	}

	// extract needed components
	if err := componentManager.Init(ctx); err != nil {
		return err
	}

	worker.KernelSetup()

	err = componentManager.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start worker components: %w", err)
	}

	if supervised := supervised.Get(ctx); supervised != nil {
		supervised.MarkReady()
	}

	// Wait for k0s process termination
	if controller != nil {
		logrus.Info("Controller has started")
	} else {
		logrus.Info("Worker has started")
	}
	<-ctx.Done()
	logrus.Info("Shutting down k0s: ", context.Cause(ctx))

	// Stop components
	if err := componentManager.Stop(); err != nil {
		logrus.WithError(err).Error("Failed to stop worker components")
	} else {
		logrus.Info("All worker components stopped")
	}
	return nil
}
