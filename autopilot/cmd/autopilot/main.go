// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/k0sproject/autopilot/pkg/client"
	apcli "github.com/k0sproject/autopilot/pkg/client"
	apconst "github.com/k0sproject/autopilot/pkg/constant"
	apcont "github.com/k0sproject/autopilot/pkg/controller"
	aproot "github.com/k0sproject/autopilot/pkg/controller/root"

	logrusr "github.com/bombsimon/logrusr/v2"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cr "sigs.k8s.io/controller-runtime"
)

func main() {
	cfg := aproot.RootConfig{}

	cmd := &cobra.Command{
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Use: apconst.AutopilotName,

		RunE: func(cmd *cobra.Command, args []string) error {
			return realMain(cfg)
		},
	}

	cmd.AddCommand(NewVersionCmd())
	// controller-runtime config loader helpers init kubeconfig go flag
	// So we need to convert it to cobra flag and add to command
	kcFlag := flag.Lookup("kubeconfig")
	cmd.Flags().AddFlag(pflag.PFlagFromGoFlag(kcFlag))

	cmd.Flags().StringVar(&cfg.Mode, "mode", "", "the runtime mode (controller, worker)")
	cmd.Flags().StringVar(&cfg.K0sDataDir, "data-dir", apconst.K0sDefaultDataDir, "k0s data dir")
	cmd.Flags().StringArrayVar(&cfg.ExcludeFromPlans, "exclude-from-plans", nil, "[mode=controller only] enforce that plans should exclude these node types (controller, worker)")
	_ = cmd.MarkFlagRequired("mode")

	cmd.Flags().IntVar(&cfg.ManagerPort, "port", 10800, "the controller-runtime manager port")
	cmd.Flags().StringVar(&cfg.MetricsBindAddr, "metrics-bind-addr", ":10801", "the controller-runtime metrics bind address")
	cmd.Flags().StringVar(&cfg.HealthProbeBindAddr, "healthz-bind-addr", ":10802", "the controller-runtime health probe bind address")

	_ = cmd.Execute()
}

func realMain(cfg aproot.RootConfig) error {
	cr.SetLogger(logrusr.New(logrus.New()))

	rootLog := setupLogger().WithField("app", "autopilot")

	config := cr.GetConfigOrDie()
	clientFactory, err := client.NewClientFactory(config)
	if err != nil {
		return fmt.Errorf("unable to build kube client factory from '%s': %w", cfg.KubeConfig, err)
	}

	root, err := createControllerRoot(cfg, rootLog, clientFactory)
	if err != nil {
		rootLog.Fatalf("Unable to start autopilot root: %v", err)
	}

	rootLog.Infof("Starting autopilot in %s mode", cfg.Mode)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return root.Run(ctx)
}

// createControllerRoot creates the appropriate root based on the mode provided.
func createControllerRoot(cfg aproot.RootConfig, logger *logrus.Entry, cf apcli.FactoryInterface) (aproot.Root, error) {
	switch cfg.Mode {
	case "controller":
		return apcont.NewRootController(cfg, logger, cf)
	case "worker":
		return apcont.NewRootWorker(cfg, logger, cf)
	}

	return nil, fmt.Errorf("unsupported root mode = '%s'", cfg.Mode)
}

// setupLogger creates a logrus.Logger configuration that matches k0s
func setupLogger() *logrus.Logger {
	return &logrus.Logger{
		Out:   os.Stdout,
		Level: logrus.InfoLevel,
		Formatter: &logrus.TextFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			FullTimestamp:   true,
		},
	}
}
