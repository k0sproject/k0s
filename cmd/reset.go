package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	resetCmd.AddCommand(resetControllerCmd)
	resetCmd.AddCommand(resetWorkerCmd)
}

var (
	resetCmd = &cobra.Command{
		Use:   "reset",
		Short: "Helper command for uninstalling k0s. Must be run as root (or with sudo)",
	}
)

var (
	resetControllerCmd = &cobra.Command{
		Use:   "controller",
		Short: "Helper command for uninstalling k0s controller node. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	resetWorkerCmd = &cobra.Command{
		Use:   "worker",
		Short: "Helper command for uninstalling k0s worker node. Must be run as root (or with sudo)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
)
