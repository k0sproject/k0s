// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta2

import (
	"testing"
	"time"

	"github.com/k0sproject/k0s/pkg/autopilot/channels"
	"github.com/stretchr/testify/require"
)

func TestPeriodicUpgradeStrategy_IsWithinPeriod(t *testing.T) {
	type fields struct {
		Days      []string
		StartTime string
		Length    string
	}
	tests := []struct {
		name   string
		fields fields
		time   time.Time
		want   bool
	}{
		{
			name: "empty days",
			fields: fields{
				Days:      []string{},
				StartTime: time.Now().Format("15:04"),
				Length:    "1h",
			},
			time: time.Now(),
			want: true,
		},
		{
			name: "Current weekday",
			fields: fields{
				Days:      []string{time.Now().Weekday().String()},
				StartTime: time.Now().Format("15:04"),
				Length:    "1h",
			},
			time: time.Now(),
			want: true,
		},
		{
			name: "Current weekday - after window",
			fields: fields{
				Days:      []string{time.Now().Weekday().String()},
				StartTime: time.Now().Format("15:04"),
				Length:    "1h",
			},
			time: time.Now().Add(time.Hour * 2),
			want: false,
		},
		{
			name: "Current weekday - before window",
			fields: fields{
				Days:      []string{time.Now().Weekday().String()},
				StartTime: time.Now().Format("15:04"),
				Length:    "1h",
			},
			time: time.Now().Add(time.Hour * -2),
			want: false,
		},
		{
			name: "Wrong weekday - outside window",
			fields: fields{
				Days:      []string{time.Now().Weekday().String()},
				StartTime: time.Now().Format("15:04"),
				Length:    "1h",
			},
			time: time.Now().Add(time.Hour * -24),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &PeriodicUpgradeStrategy{
				Days:      tt.fields.Days,
				StartTime: tt.fields.StartTime,
				Length:    tt.fields.Length,
			}
			if got := p.IsWithinPeriod(tt.time); got != tt.want {
				t.Errorf("PeriodicUpgradeStrategy.IsWithinPeriod() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestToPlan_EmptyCommand(t *testing.T) {
	uc := UpdateConfig{
		Spec: UpdateSpec{
			PlanSpec: AutopilotPlanSpec{},
		},
	}

	nextVersion := channels.VersionInfo{
		Version: "v1.2.3",
		DownloadURLs: []channels.DownloadURL{
			{
				Arch: "arm64",
				OS:   "linux",
				K0S:  "some_k0s_url",
			},
		},
	}
	plan := uc.ToPlan(nextVersion)
	require := require.New(t)
	var k0sCommand *PlanCommandK0sUpdate
	for _, c := range plan.Spec.Commands {
		if c.K0sUpdate != nil {
			k0sCommand = c.K0sUpdate
		}
	}
	require.Equal("some_k0s_url", k0sCommand.Platforms["linux-arm64"].URL)
}

func TestToPlan_ExistingCommand(t *testing.T) {
	uc := UpdateConfig{
		Spec: UpdateSpec{
			PlanSpec: AutopilotPlanSpec{
				Commands: []AutopilotPlanCommand{
					{
						K0sUpdate: &AutopilotPlanCommandK0sUpdate{
							ForceUpdate: true,
							Targets: PlanCommandTargets{
								Controllers: PlanCommandTarget{
									Discovery: PlanCommandTargetDiscovery{
										Selector: nil,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	nextVersion := channels.VersionInfo{
		Version: "v1.2.3",
		DownloadURLs: []channels.DownloadURL{
			{
				Arch: "arm64",
				OS:   "linux",
				K0S:  "some_k0s_url",
			},
		},
	}
	plan := uc.ToPlan(nextVersion)
	require := require.New(t)
	var k0sCommand *PlanCommandK0sUpdate
	for _, c := range plan.Spec.Commands {
		if c.K0sUpdate != nil {
			k0sCommand = c.K0sUpdate
		}
	}
	require.Equal("some_k0s_url", k0sCommand.Platforms["linux-arm64"].URL)

}
