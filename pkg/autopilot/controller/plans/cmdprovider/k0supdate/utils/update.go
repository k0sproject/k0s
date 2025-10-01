// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"fmt"
	"time"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
	apsigv2 "github.com/k0sproject/k0s/pkg/autopilot/signaling/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crcli "sigs.k8s.io/controller-runtime/pkg/client"
)

type SignalNodeCommandBuilder func() apsigv2.Command

// UpdateSignalNode builds a signaling update request, and adds it to the provided node
func UpdateSignalNode(node crcli.Object, planID string, cb SignalNodeCommandBuilder) error {
	signalData := apsigv2.SignalData{
		PlanID:  planID,
		Created: time.Now().Format(time.RFC3339),
		Command: cb(),
	}

	if err := signalData.Validate(); err != nil {
		return fmt.Errorf("unable to validate signaling data: %w", err)
	}

	if node.GetAnnotations() == nil {
		node.SetAnnotations(make(map[string]string))
	}

	if err := signalData.Marshal(node.GetAnnotations()); err != nil {
		return fmt.Errorf("unable to marshal signaling data: %w", err)
	}

	return nil
}

func UpdatePlanCommandTargetStatusByName(name string, status apv1beta2.PlanCommandTargetStateType, pcts []apv1beta2.PlanCommandTargetStatus) bool {
	for idx, node := range pcts {
		if node.Name == name {
			pcts[idx].State = status
			pcts[idx].LastUpdatedTimestamp = metav1.Now()
			return true
		}
	}

	return false
}
