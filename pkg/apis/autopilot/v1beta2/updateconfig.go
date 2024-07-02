// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta2

import (
	"fmt"
	"strconv"
	"time"

	uc "github.com/k0sproject/k0s/pkg/autopilot/channels"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const UpdateConfigFinalizer = "updateconfig.autopilot.k0sproject.io"

const (
	UpdateStrategyTypeCron     = "cron"
	UpdateStrategyTypePeriodic = "periodic"
)

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +genclient
// +genclient:onlyVerbs=create,delete,list,get,watch,update
// +genclient:nonNamespaced
type UpdateConfig struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	metav1.TypeMeta   `json:",omitempty,inline"`

	Spec UpdateSpec `json:"spec"`
}

type UpdateSpec struct {
	// Channel defines the update channel to use for this update config
	// +kubebuilder:default:=stable
	Channel string `json:"channel,omitempty"`
	// UpdateServer defines the update server to use for this update config
	// +kubebuilder:default:="https://updates.k0sproject.io"
	UpdateServer string `json:"updateServer,omitempty"`
	// UpdateStrategy defines the update strategy to use for this update config
	UpgradeStrategy UpgradeStrategy `json:"upgradeStrategy,omitempty"`
	// PlanSpec defines the plan spec to use for this update config
	PlanSpec AutopilotPlanSpec `json:"planSpec,omitempty"`
}

// AutopilotPlanSpec describes the behavior of the autopilot generated `Plan`
type AutopilotPlanSpec struct {
	// Commands are a collection of all of the commands that need to be executed
	// in order for this plan to transition to Completed.
	Commands []AutopilotPlanCommand `json:"commands"`
}

// AutopilotPlanCommand is a command that can be run within a `Plan`
type AutopilotPlanCommand struct {
	// K0sUpdate is the `K0sUpdate` command which is responsible for updating a k0s node (controller/worker)
	K0sUpdate *AutopilotPlanCommandK0sUpdate `json:"k0supdate,omitempty"`

	// AirgapUpdate is the `AirgapUpdate` command which is responsible for updating a k0s airgap bundle.
	AirgapUpdate *AutopilotPlanCommandAirgapUpdate `json:"airgapupdate,omitempty"`
}

// AutopilotPlanCommandK0sUpdate provides all of the information to for a `K0sUpdate` command to
// update a set of target signal nodes.
type AutopilotPlanCommandK0sUpdate struct {
	// ForceUpdate ensures that version checking is ignored and that all updates are applied.
	ForceUpdate bool `json:"forceupdate,omitempty"`

	// Targets defines how the controllers/workers should be discovered and upgraded.
	Targets PlanCommandTargets `json:"targets"`
}

// AutopilotPlanCommandAirgapUpdate provides all of the information to for a `AirgapUpdate` command to
// update a set of target signal nodes
type AutopilotPlanCommandAirgapUpdate struct {
	// Workers defines how the k0s workers will be discovered and airgap updated.
	Workers PlanCommandTarget `json:"workers"`
}

type UpgradeStrategy struct {
	// Type defines the type of upgrade strategy
	// +kubebuilder:validation:Enum=periodic;cron
	Type string `json:"type,omitempty"`
	// Cron defines the cron expression for the cron upgrade strategy
	// Deprecated: Cron is deprecated and will eventually be ignored
	Cron string `json:"cron,omitempty"`
	// Periodic defines the periodic upgrade strategy
	Periodic PeriodicUpgradeStrategy `json:"periodic,omitempty"`
}

type PeriodicUpgradeStrategy struct {
	Days      []string `json:"days,omitempty"`
	StartTime string   `json:"startTime,omitempty"`
	Length    string   `json:"length,omitempty"`
}

func (p *PeriodicUpgradeStrategy) IsWithinPeriod(t time.Time) bool {
	days := p.Days
	if len(p.Days) == 0 {
		days = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	}

	// Parse the start time and window length
	st, err := time.Parse("15:04", p.StartTime)
	if err != nil {
		fmt.Println("Error parsing start time:", err)
		return false
	}

	startTime := startTimeForCurrentDay(st)

	windowDuration, err := time.ParseDuration(p.Length)
	if err != nil {
		fmt.Println("Error parsing window length:", err)
		return false
	}

	// Check if the current day is within the specified window days
	currentDay := t.Weekday().String()
	isWindowDay := false
	for _, day := range days {
		if day == currentDay {
			isWindowDay = true
			break
		}
	}

	// Check if the current time is within the specified window
	return isWindowDay &&
		t.After(startTime) &&
		t.Before(startTime.Add(windowDuration))

}

// Returns the "adjusted" time for the current day. I.e. if the starTime is 15:00, this function will return the current day at 15:00
func startTimeForCurrentDay(startTime time.Time) time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), startTime.Hour(), startTime.Minute(), 0, 0, time.Local)
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type UpdateConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UpdateConfig `json:"items"`
}

func (uc *UpdateConfig) ToPlan(nextVersion uc.VersionInfo) Plan {
	p := Plan{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Plan",
			APIVersion: "autopilot.k0sproject.io/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "autopilot",
		},
		Spec: PlanSpec{},
	}

	platforms := make(PlanPlatformResourceURLMap)
	airgapPlatforms := make(PlanPlatformResourceURLMap)
	for _, downloadURL := range nextVersion.DownloadURLs {
		osArch := fmt.Sprintf("%s-%s", downloadURL.OS, downloadURL.Arch)
		k0sURL := PlanResourceURL{
			URL: downloadURL.K0S,
		}
		if downloadURL.K0SSha256 != "" {
			k0sURL.Sha256 = downloadURL.K0SSha256
		}
		platforms[osArch] = k0sURL

		airgapURL := PlanResourceURL{
			URL: downloadURL.AirgapBundle,
		}
		if downloadURL.AirgapSha256 != "" {
			airgapURL.Sha256 = downloadURL.AirgapSha256
		}
		airgapPlatforms[osArch] = airgapURL
	}

	p.Spec.ID = strconv.FormatInt(time.Now().Unix(), 10)
	p.Spec.Timestamp = strconv.FormatInt(time.Now().Unix(), 10)

	var updateCommandFound bool
	for _, cmd := range uc.Spec.PlanSpec.Commands {
		if cmd.K0sUpdate != nil || cmd.AirgapUpdate != nil {
			updateCommandFound = true
			break
		}
	}

	// If update command is not specified, we add a default one to update all controller and workers in the cluster
	if !updateCommandFound {
		p.Spec.Commands = append(p.Spec.Commands, PlanCommand{
			K0sUpdate: &PlanCommandK0sUpdate{
				Version:   string(nextVersion.Version),
				Platforms: platforms,
				Targets: PlanCommandTargets{
					Controllers: PlanCommandTarget{
						Discovery: PlanCommandTargetDiscovery{
							Selector: &PlanCommandTargetDiscoverySelector{},
						},
					},
					Workers: PlanCommandTarget{
						Discovery: PlanCommandTargetDiscovery{
							Selector: &PlanCommandTargetDiscoverySelector{},
						},
					},
				},
			},
		})
	} else {
		for _, cmd := range uc.Spec.PlanSpec.Commands {
			planCmd := PlanCommand{}
			if cmd.K0sUpdate != nil {
				planCmd.K0sUpdate = &PlanCommandK0sUpdate{
					Version:     string(nextVersion.Version),
					ForceUpdate: cmd.K0sUpdate.ForceUpdate,
					Platforms:   platforms,
					Targets:     cmd.K0sUpdate.Targets,
				}
			}
			if cmd.AirgapUpdate != nil {
				planCmd.AirgapUpdate = &PlanCommandAirgapUpdate{
					Version:   string(nextVersion.Version),
					Platforms: airgapPlatforms,
					Workers:   cmd.AirgapUpdate.Workers,
				}
			}
			p.Spec.Commands = append(p.Spec.Commands, planCmd)
		}
	}

	return p
}
