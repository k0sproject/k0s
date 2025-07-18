// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package node

import (
	"context"
	"fmt"

	apitypes "k8s.io/apimachinery/pkg/types"
	nodeutil "k8s.io/component-helpers/node/util"
)

// GetNodeName returns the node name for the node taking OS, cloud provider and override into account
func GetNodeName(override string) (apitypes.NodeName, error) {
	return getNodeName(context.TODO(), override)
}

func getNodeName(ctx context.Context, override string) (apitypes.NodeName, error) {
	if override == "" {
		var err error
		override, err = defaultNodeNameOverride(ctx)
		if err != nil {
			return "", err
		}
	}
	nodeName, err := nodeutil.GetHostname(override)
	if err != nil {
		return "", fmt.Errorf("failed to determine node name: %w", err)
	}
	return apitypes.NodeName(nodeName), nil
}
