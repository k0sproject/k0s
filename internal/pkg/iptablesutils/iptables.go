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

package iptablesutils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	ModeNFT    = "nft"
	ModeLegacy = "legacy"
)

// DetectIPTablesMode figure out whether iptables-legacy or iptables-nft is in use on the host.
// Follows the same logic as kube-proxy/kube-route.
// See: https://github.com/kubernetes-sigs/iptables-wrappers/blob/master/iptables-wrapper-installer.sh
func DetectIPTablesMode(k0sBinPath string) (string, error) {
	logrus.Info("Trying to detect iptables mode")
	logrus.Info("Checking iptables-nft for kube-related entries")
	nftEntriesCount, nftTotalCount, err := iptablesEntriesCount(k0sBinPath, "xtables-nft-multi", []string{"KUBE-IPTABLES-HINT", "KUBE-KUBELET-CANARY"})
	if err != nil {
		return "", fmt.Errorf("error checking iptables-nft entries: %w", err)
	}
	if nftEntriesCount > 0 {
		logrus.Info("kube-related iptables entries found for iptables-nft")
		return ModeNFT, nil
	}

	logrus.Info("Checking iptables-legacy for kube-related entries")
	legacyEntriesCount, legacyTotalCount, err := iptablesEntriesCount(k0sBinPath, "xtables-legacy-multi", []string{"KUBE-IPTABLES-HINT", "KUBE-KUBELET-CANARY"})
	if err != nil {
		return "", fmt.Errorf("error checking iptables-legacy entries: %w", err)
	}
	if legacyEntriesCount > 0 {
		logrus.Info("kube-related iptables entries found for iptables-legacy")
		return ModeLegacy, nil
	}

	if legacyTotalCount > nftTotalCount {
		logrus.Info("kube-related iptables entries not found, go with iptables-legacy")
		return ModeLegacy, nil
	}
	logrus.Info("kube-related iptables entries not found, go with iptables-nft")
	return ModeNFT, nil
}

func iptablesEntriesCount(k0sBinPath string, xtablesCmdName string, entries []string) (entriesFound int, total int, err error) {
	xtablesBinPath := filepath.Join(k0sBinPath, xtablesCmdName)

	cmdString := fmt.Sprintf("(%s iptables-save || true; %s ip6tables-save || true) 2>/dev/null", xtablesBinPath, xtablesBinPath)
	cmd := exec.Command("/bin/sh", "-c", cmdString)

	var out bytes.Buffer
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		return 0, 0, err
	}

	scanner := bufio.NewScanner(&out)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		total++
		line := scanner.Text()
		for _, entry := range entries {
			if strings.Contains(line, entry) {
				entriesFound++
			}
		}
	}

	return entriesFound, total, nil
}
