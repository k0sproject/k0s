// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package keepalived

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

func NewKeepalivedSetStateCmd() *cobra.Command {
	var (
		rundir     string
		state      string
		iface      string
		virtualIPs []string
	)

	cmd := &cobra.Command{
		Use:   "keepalived-setstate",
		Short: "Set keepalived state",
		Long: `Example:
   k0s keepalived-setstate -r <rundir> -s <state>`,
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			rundir = unescapeSingleQuotes(rundir)
			// Verify that rundir is a valid directory
			if err := validateRundir(rundir); err != nil {
				return err
			}

			lock, err := os.OpenFile(filepath.Join(rundir, "keepalived-setstate.lock"), os.O_CREATE|os.O_RDWR, 0600)
			if err != nil {
				return err
			}
			defer lock.Close()
			if err = unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
				return fmt.Errorf("failed to lock %s: %w", lock.Name(), err)
			}
			if iface != "" {
				if state, err = stateOf(iface, virtualIPs); err != nil {
					return err
				}
			}

			// generatedVirtualServers doesn't need to exist in order to be linked
			// so we don't need to check for it.
			// If it doesn't exist yet, we link it now and when k0s creates it, it
			// will signal keepalived.
			generatedVirtualServers := filepath.Join(rundir, "keepalived-virtualservers-generated.conf")

			sourceFile := ""
			switch state {
			case "MASTER":
				sourceFile = generatedVirtualServers
			case "BACKUP":
				sourceFile = os.DevNull
			default:
				return fmt.Errorf("invalid state %s, expected MASTER or BACKUP", state)

			}

			if err = createSymlink(rundir, sourceFile); err != nil {
				return fmt.Errorf("failed to create symlink: %w", err)
			}

			pid, err := getPid(rundir)
			if err != nil {
				return err
			}

			if err := syscall.Kill(pid, syscall.SIGHUP); err != nil {
				return fmt.Errorf("failed to send SIGHUP to pid %d: %w", pid, err)
			}
			return nil
		},
	}
	// Add flags
	cmd.Flags().StringVarP(&rundir, "run-dir", "r", "", "Path to the run-dir (required)")
	cmd.Flags().StringVarP(&state, "state", "s", "", "State to set (MASTER or BACKUP) (required)")
	cmd.Flags().StringVar(&iface, "interface", "", "Interface of the VRRP instance, if set the state is MASTER when it holds any --virtual-ip and BACKUP otherwise")
	cmd.Flags().StringArrayVar(&virtualIPs, "virtual-ip", nil, "Virtual IP of the VRRP instance, in CIDR notation")
	cmd.MarkFlagsRequiredTogether("interface", "virtual-ip")

	// Mark flags as required
	_ = cmd.MarkFlagRequired("rundir")
	_ = cmd.MarkFlagRequired("state")

	return cmd
}

func stateOf(iface string, virtualIPs []string) (string, error) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return "", fmt.Errorf("failed to get interface %s: %w", iface, err)
	}
	addrs, err := link.Addrs()
	if err != nil {
		return "", fmt.Errorf("failed to list the addresses of %s: %w", iface, err)
	}
	return stateFromAddrs(addrs, virtualIPs)
}

func stateFromAddrs(addrs []net.Addr, virtualIPs []string) (string, error) {
	for _, virtualIP := range virtualIPs {
		ip, _, err := net.ParseCIDR(virtualIP)
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.Equal(ip) {
				return "MASTER", nil
			}
		}
	}
	return "BACKUP", nil
}

func createSymlink(rundir string, sourceFile string) error {
	consumedVirtualServers := filepath.Join(rundir, "keepalived-virtualservers-consumed.conf")

	if err := os.Remove(consumedVirtualServers); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove consumed virtual servers path %q: %w", consumedVirtualServers, err)
	}

	if err := os.Symlink(sourceFile, consumedVirtualServers); err != nil {
		return fmt.Errorf("failed to create soft link from %q to %q: %w", sourceFile, consumedVirtualServers, err)
	}
	return nil
}

func validateRundir(rundir string) error {
	path, err := os.Stat(rundir)
	if err != nil {
		return fmt.Errorf("run-dir %q is invalid: %w", rundir, err)
	}
	if !path.IsDir() {
		return fmt.Errorf("run-dir %q is not a directory", rundir)
	}
	return nil
}

func getPid(rundir string) (int, error) {
	pidfile := filepath.Join(rundir, "keepalived.pid")
	pidBytes, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, fmt.Errorf("failed to read pidfile %q: %w", pidfile, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		return 0, fmt.Errorf("failed to convert pid %q to int: %w", pidBytes, err)
	}
	return pid, nil

}

func unescapeSingleQuotes(s string) string {
	// Replace escaped single quotes with a single quote
	return strings.ReplaceAll(s, `\'`, `'`)
}
