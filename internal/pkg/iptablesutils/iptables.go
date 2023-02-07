/*
Copyright 2023 k0s authors

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

package iptablesutils

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
	"go.uber.org/multierr"
)

const (
	ModeNFT    = "nft"
	ModeLegacy = "legacy"
)

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
		return "", multierr.Combine(err, nftErr, legacyErr)
	}

	out, err := exec.Command(iptablesPath, "--version").CombinedOutput()
	if err != nil {
		return "", multierr.Combine(err, nftErr, legacyErr)
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
		return false, 0, multierr.Combine(
			fmt.Errorf("iptables-save: %w", v4Err),
			fmt.Errorf("ip6tables-save: %w", v6Err),
		)
	}

	return entriesFound, total, nil
}
