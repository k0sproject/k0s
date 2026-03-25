//go:build windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

// Run removes Windows CNI artifacts.
func (c *cni) Run() error {
	removeCNIConfigFiles()
	removeCalicoServices()
	cleanupHNSArtifacts()
	cleanupVethernetAdapters()
	cleanupCalicoEnvVars()
	cleanupCalicoFirewallRules()
	return nil
}

// removeCNIConfigFiles deletes all known Windows CNI config artifacts from disk.
func removeCNIConfigFiles() {
	logrus.Debug("removing Windows CNI configuration files")
	var errs []error

	files := []string{
		`C:\\etc\\cni\\net.d\\10-calico.conflist`,
		`C:\\etc\\cni\\net.d\\calico-kubeconfig`,
		`C:\\etc\\cni\\net.d\\10-kuberouter.conflist`,
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil && !errors.Is(err, fs.ErrNotExist) {
			logrus.WithError(err).Warnf("failed to remove Windows CNI configuration file %s", filepath.Clean(f))
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		aggErr := errors.Join(errs...)
		logrus.WithError(aggErr).Warn("Windows CNI cleanup: removing configuration files incomplete")
	}
}

// removeCalicoServices stops and deletes Calico-related Windows services.
func removeCalicoServices() {
	logrus.Debug("removing Windows Calico services")
	services := []string{"CalicoNode", "CalicoFelix", "CalicoConfd"}
	var errs []error
	for _, name := range services {
		logrus.Debugf("removing Windows service %s", name)
		script := fmt.Sprintf("$svc = Get-Service -Name '%s' -ErrorAction SilentlyContinue; if (-not $svc) { exit 0 }; if ($svc.Status -ne 'Stopped') { Stop-Service -Name '%s' -Force -Confirm:$false -ErrorAction SilentlyContinue }; sc.exe delete '%s' | Out-Null", name, name, name)
		if err := runPowerShell(script); err != nil {
			logrus.WithError(err).Warnf("failed to remove Windows service %s", name)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		joinedErr := errors.Join(errs...)
		logrus.WithError(joinedErr).Warn("Windows CNI cleanup: removing Calico services incomplete")
	}
}

// cleanupHNSArtifacts clears Calico/External HNS networks and their endpoints.
func cleanupHNSArtifacts() {
	logrus.Debug("cleaning up Windows HNS artifacts")
	script := strings.Join([]string{
		`$ensureHnsCmdlets = {`,
		`    $hasGetNetwork = Get-Command -Name Get-HnsNetwork -ErrorAction SilentlyContinue`,
		`    $hasRemoveNetwork = Get-Command -Name Remove-HnsNetwork -ErrorAction SilentlyContinue`,
		`    $hasGetEndpoint = Get-Command -Name Get-HnsEndpoint -ErrorAction SilentlyContinue`,
		`    $hasRemoveEndpoint = Get-Command -Name Remove-HnsEndpoint -ErrorAction SilentlyContinue`,
		`    if (-not ($hasGetNetwork -and $hasRemoveNetwork -and $hasGetEndpoint -and $hasRemoveEndpoint)) {`,
		`        $defaultModule = Join-Path $env:SystemRoot 'System32\\WindowsPowerShell\\v1.0\\Modules\\hns\\hns.psm1'`,
		`        if (Test-Path $defaultModule) {`,
		`            try { Import-Module $defaultModule -ErrorAction Stop } catch { }`,
		`        }`,
		`        $hasGetNetwork = Get-Command -Name Get-HnsNetwork -ErrorAction SilentlyContinue`,
		`        $hasRemoveNetwork = Get-Command -Name Remove-HnsNetwork -ErrorAction SilentlyContinue`,
		`        $hasGetEndpoint = Get-Command -Name Get-HnsEndpoint -ErrorAction SilentlyContinue`,
		`        $hasRemoveEndpoint = Get-Command -Name Remove-HnsEndpoint -ErrorAction SilentlyContinue`,
		`    }`,
		`    return $hasGetNetwork -and $hasRemoveNetwork -and $hasGetEndpoint -and $hasRemoveEndpoint`,
		`}`,
		`$cmdletsAvailable = & $ensureHnsCmdlets`,
		`if (-not $cmdletsAvailable) {`,
		`    Write-Output 'k0s: skipping HNS cleanup because cmdlets are unavailable'`,
		`    return`,
		`}`,
		`$targetNetworks = Get-HnsNetwork | Where-Object { $_.Name -like '*calico*' -or $_.Name -eq 'External' }`,
		`foreach ($net in $targetNetworks) {`,
		`    $netId = $net.Id`,
		`    if (-not $netId) { continue }`,
		`    Get-HnsEndpoint | Where-Object { $_.VirtualNetwork -eq $netId -or $_.Name -like '*calico*' } | Remove-HnsEndpoint -ErrorAction SilentlyContinue`,
		`    $net | Remove-HnsNetwork -ErrorAction SilentlyContinue | Out-Null`,
		`}`,
		`Get-HnsEndpoint | Where-Object { $_.Name -like '*calico*' } | Remove-HnsEndpoint -ErrorAction SilentlyContinue`,
	}, "; ")
	if err := runPowerShell(script); err != nil {
		logrus.WithError(err).Warn("failed to remove Windows HNS networks and endpoints")
	}
}

// cleanupVethernetAdapters removes the vEthernet adapters created for containers.
func cleanupVethernetAdapters() {
	logrus.Debug("removing Windows vEthernet adapters")
	script := strings.Join([]string{
		`$getNetAdapterCmd = Get-Command -Name Get-NetAdapter -ErrorAction SilentlyContinue`,
		`$removeNetAdapterCmd = Get-Command -Name Remove-NetAdapter -ErrorAction SilentlyContinue`,
		`if ($getNetAdapterCmd -and $removeNetAdapterCmd) {`,
		`    Get-NetAdapter | Where-Object { $_.Name -like '*calico*' -or $_.Name -like 'vEthernet (Container NIC*' } | Remove-NetAdapter -Confirm:$false -ErrorAction SilentlyContinue`,
		`}`,
	}, "; ")
	if err := runPowerShell(script); err != nil {
		logrus.WithError(err).Warn("failed to remove Windows vEthernet adapters")
	}
}

// cleanupCalicoEnvVars removes any Calico/Felix environment variables.
func cleanupCalicoEnvVars() {
	logrus.Debug("removing Windows Calico/Felix environment variables")
	script := strings.Join([]string{
		`$envPath = 'HKLM:\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment'`,
		`$machineVars = [System.Environment]::GetEnvironmentVariables('Machine')`,
		`foreach ($key in $machineVars.Keys) {`,
		`    if ($key -like 'CALICO_*' -or $key -like 'FELIX_*') {`,
		`        Remove-ItemProperty -LiteralPath $envPath -Name $key -ErrorAction SilentlyContinue`,
		`    }`,
		`}`,
	}, "; ")
	if err := runPowerShell(script); err != nil {
		logrus.WithError(err).Warn("failed to remove Windows Calico/Felix environment variables")
	}
}

// cleanupCalicoFirewallRules deletes firewall rules created by Calico.
func cleanupCalicoFirewallRules() {
	logrus.Debug("removing Windows Calico firewall rules")
	script := `Get-NetFirewallRule | Where-Object { $_.DisplayName -like '*calico*' } | Remove-NetFirewallRule -ErrorAction SilentlyContinue`
	if err := runPowerShell(script); err != nil {
		logrus.WithError(err).Warn("failed to remove Windows Calico firewall rules")
	}
}

// runPowerShell executes a PowerShell script and wraps any errors with output.
func runPowerShell(script string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	trimmedStdErr := strings.TrimSpace(stderr.String())
	if trimmedStdErr != "" {
		log := logrus.WithError(err)
		log.Debugf("PowerShell stderr: %s", trimmedStdErr)
		if err != nil {
			return fmt.Errorf("%s: %w", trimmedStdErr, err)
		}
		return errors.New(trimmedStdErr)
	}
	if err != nil {
		trimmedStdOut := strings.TrimSpace(stdout.String())
		if trimmedStdOut != "" {
			logrus.WithError(err).Debugf("PowerShell error: %s", trimmedStdOut)
			return fmt.Errorf("%s: %w", trimmedStdOut, err)
		}
		logrus.WithError(err).Debug("PowerShell command failed without output")
		return err
	}
	return nil
}
