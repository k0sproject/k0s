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

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/spf13/cobra"
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

			if state != "MASTER" && state != "BACKUP" {
				return fmt.Errorf("invalid state %s, expected MASTER or BACKUP", state)
			}

			return setState(rundir, state, iface, virtualIPs)
		},
	}
	// Add flags
	cmd.Flags().StringVarP(&rundir, "run-dir", "r", "", "Path to the run-dir (required)")
	cmd.Flags().StringVarP(&state, "state", "s", "", "State to set (MASTER or BACKUP) (required)")
	cmd.Flags().StringVar(&iface, "interface", "", "Interface of the VRRP instance, decides the state with --virtual-ip")
	cmd.Flags().StringArrayVar(&virtualIPs, "virtual-ip", nil, "Virtual IP of the VRRP instance, in CIDR notation")

	// Mark flags as required
	_ = cmd.MarkFlagRequired("rundir")
	_ = cmd.MarkFlagRequired("state")

	return cmd
}

// setState links the virtual servers for MASTER and nothing for BACKUP, then reloads keepalived.
// Notify scripts can run at the same time, so a lock runs them one by one and the virtual IPs decide.
func setState(rundir, state, iface string, virtualIPs []string) error {
	lockFile, err := file.LockPath(filepath.Join(rundir, "keepalived-setstate.lock"), true)
	if err != nil {
		return err
	}
	// Closing the file releases the lock.
	defer func() { _ = lockFile.Close() }()

	master := state == "MASTER"
	if iface != "" && len(virtualIPs) > 0 {
		if master, err = interfaceHoldsVirtualIP(iface, virtualIPs); err != nil {
			return err
		}
	}

	// The generated file may not exist yet. k0s reloads keepalived once it writes the file.
	sourceFile := os.DevNull
	if master {
		sourceFile = filepath.Join(rundir, "keepalived-virtualservers-generated.conf")
	}

	if err = createSymlink(rundir, sourceFile); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return reloadKeepalived(rundir)
}

// interfaceHoldsVirtualIP reports whether any of the virtual IPs is on the interface.
func interfaceHoldsVirtualIP(iface string, virtualIPs []string) (bool, error) {
	link, err := net.InterfaceByName(iface)
	if err != nil {
		return false, err
	}
	addrs, err := link.Addrs()
	if err != nil {
		return false, fmt.Errorf("failed to list the addresses of %s: %w", iface, err)
	}
	return holdsVirtualIP(virtualIPs, addrs)
}

// reloadKeepalived makes keepalived reload its configuration.
func reloadKeepalived(rundir string) error {
	pid, err := getPid(rundir)
	if err != nil {
		return err
	}

	if err = syscall.Kill(pid, syscall.SIGHUP); err != nil {
		return fmt.Errorf("failed to send SIGHUP to pid %d: %w", pid, err)
	}
	return nil
}

// holdsVirtualIP reports whether any of the virtual IPs is among the addresses.
func holdsVirtualIP(virtualIPs []string, addrs []net.Addr) (bool, error) {
	for _, virtualIP := range virtualIPs {
		ip, _, err := net.ParseCIDR(virtualIP)
		if err != nil {
			return false, err
		}
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.Equal(ip) {
				return true, nil
			}
		}
	}
	return false, nil
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
