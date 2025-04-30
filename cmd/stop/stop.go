/*
Copyright 2021 k0s authors

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

package stop

import (
	"errors"
	"os"
	"runtime"

	"github.com/k0sproject/k0s/cmd/internal"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

func NewStopCmd() *cobra.Command {
	var debugFlags internal.DebugFlags

	cmd := &cobra.Command{
		Use:              "stop",
		Short:            "Stop the k0s service configured on this host. Must be run as root (or with sudo)",
		Args:             cobra.NoArgs,
		PersistentPreRun: debugFlags.Run,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if runtime.GOOS != "windows" && os.Geteuid() != 0 {
				return errors.New("this command must be run as root")
			}
			svc, err := install.InstalledService()
			if err != nil {
				return err
			}
			status, err := svc.Status()
			if err != nil {
				return err
			}
			if status == service.StatusStopped {
				return errors.New("already stopped")
			}
			return svc.Stop()
		},
	}

	debugFlags.AddToFlagSet(cmd.PersistentFlags())

	return cmd
}
