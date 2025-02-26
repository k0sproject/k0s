/*
Copyright 2024 k0s authors

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

package iptables

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/k0sproject/k0s/pkg/assets"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/sirupsen/logrus"
)

const (
	ModeNFT    = "nft"
	ModeLegacy = "legacy"
)

type Component struct {
	IPTablesMode string
	BinDir       string
}

func (c *Component) Init(_ context.Context) error {
	log := logrus.WithField("component", constant.IptablesBinariesComponentName)
	log.Info("Staging iptables binaries")
	err, iptablesMode := extractIPTablesBinaries(c.BinDir, c.IPTablesMode)
	if err != nil {
		return err
	}

	c.IPTablesMode = iptablesMode
	log.Infof("iptables mode: %s", c.IPTablesMode)
	return nil
}

func (c *Component) Start(_ context.Context) error {
	return nil
}

func (c *Component) Stop() error {
	return nil
}

// extractIPTablesBinaries extracts the iptables binaries from the k0s binary and makes the symlinks
// to the backend detected by DetectHostIPTablesMode.
// extractIPTablesBinaries only works on linux, if called in another OS it will return an error.
func extractIPTablesBinaries(k0sBinDir string, iptablesMode string) (error, string) {
	cmds := []string{"xtables-legacy-multi", "xtables-nft-multi"}
	for _, cmd := range cmds {
		err := assets.Stage(k0sBinDir, cmd)
		if err != nil {
			return err, ""
		}
	}
	if iptablesMode == "" || iptablesMode == "auto" {
		var err error
		iptablesMode, err = DetectHostIPTablesMode(k0sBinDir)
		if err != nil {
			if kernelMajorVersion() < 5 {
				iptablesMode = ModeLegacy
			} else {
				iptablesMode = ModeNFT
			}
			logrus.WithError(err).Infof("Failed to detect iptables mode, using iptables-%s by default", iptablesMode)
		}
	}
	logrus.Infof("using iptables-%s", iptablesMode)
	oldpath := fmt.Sprintf("xtables-%s-multi", iptablesMode)
	for _, symlink := range []string{"iptables", "iptables-save", "iptables-restore", "ip6tables", "ip6tables-save", "ip6tables-restore"} {
		symlinkPath := filepath.Join(k0sBinDir, symlink)

		// remove if it exist and ignore error if it doesn't
		_ = os.Remove(symlinkPath)

		err := os.Symlink(oldpath, symlinkPath)
		if err != nil {
			return fmt.Errorf("failed to create symlink %s: %w", symlink, err), ""
		}
	}

	return nil, iptablesMode
}
func kernelMajorVersion() byte {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err != nil {
		return 0
	}
	return data[0] - '0'
}

// DetectHostIPTablesMode figure out whether iptables-legacy or iptables-nft is in use on the host.
// Follows the same logic as kube-proxy/kube-route.
// See: https://github.com/kubernetes-sigs/iptables-wrappers/blob/master/iptables-wrapper-installer.sh
func DetectHostIPTablesMode(k0sBinPath string) (string, error) {
	logrus.Info("Trying to detect iptables mode")

	nftMatches, nftTotal, nftErr := findMatchingEntries(k0sBinPath, ModeNFT, "KUBE-IPTABLES-HINT", "KUBE-KUBELET-CANARY")
	if nftErr != nil {
		logrus.WithError(nftErr).Debug("Failed to inspect iptables rules in nft mode")
		nftErr = fmt.Errorf("nft: %w", nftErr)
	} else if nftMatches {
		logrus.Infof("Some kube-related iptables entries found for iptables-nft")
		return ModeNFT, nil
	}

	legacyMatches, legacyTotal, legacyErr := findMatchingEntries(k0sBinPath, ModeLegacy, "KUBE-IPTABLES-HINT", "KUBE-KUBELET-CANARY")
	if legacyErr != nil {
		logrus.WithError(legacyErr).Debug("Failed to inspect iptables rules in legacy mode")
		legacyErr = fmt.Errorf("legacy: %w", legacyErr)
	} else if legacyMatches {
		logrus.Infof("Some kube-related iptables entries found for iptables-legacy")
		return ModeLegacy, nil
	}

	if nftErr == nil && legacyErr == nil && legacyTotal > nftTotal {
		logrus.Infof(
			"No kube-related entries found in neither iptables-nft nor iptables-legacy, "+
				"but there are more legacy entries than nft entries (%d vs. %d), "+
				"so go with iptables-legacy",
			legacyTotal, nftTotal,
		)
		return ModeLegacy, nil
	}

	iptablesPath, err := exec.LookPath("iptables")
	if err != nil {
		return "", errors.Join(err, nftErr, legacyErr)
	}

	out, err := exec.Command(iptablesPath, "--version").CombinedOutput()
	if err != nil {
		return "", errors.Join(err, nftErr, legacyErr)
	}

	outStr := strings.TrimSpace(string(out))
	mode := ModeLegacy

	if strings.Contains(outStr, "nf_tables") {
		mode = ModeNFT
	}

	logrus.Infof("Selecting iptables-%s: %s --version: %s", mode, iptablesPath, outStr)
	return mode, nil
}

func findMatchingEntries(k0sBinPath, mode string, entries ...string) (entriesFound bool, total uint, _ error) {
	binaryPath := filepath.Join(k0sBinPath, fmt.Sprintf("xtables-%s-multi", mode))

	findMatches := func(subcommand string) error {
		cmd := exec.Command(binaryPath, subcommand)
		out, err := cmd.StdoutPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}

		scanner := bufio.NewScanner(out)
		scanner.Split(bufio.ScanLines)
		for scanner.Scan() {
			total++
			if !entriesFound {
				line := scanner.Text()
				for _, entry := range entries {
					if strings.Contains(line, entry) {
						entriesFound = true
					}
				}
			}
		}

		return cmd.Wait()
	}

	v4Err, v6Err := findMatches("iptables-save"), findMatches("ip6tables-save")
	if v4Err != nil && v6Err != nil {
		return false, 0, fmt.Errorf("iptables-save: %w; ip6tables-save: %w", v4Err, v6Err)
	}

	return entriesFound, total, nil
}
