// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"strconv"

	"github.com/k0sproject/k0s/pkg/component/worker/containerd"

	"github.com/spf13/pflag"
)

// Options for confining containerd by AppArmor.
type AppArmorFlags struct {
	ContainerdProfileManagement containerd.AppArmorProfileManagement
	ContainerdProfileName       string
}

func (o *AppArmorFlags) AddToFlagSet(flags *pflag.FlagSet) {
	o.ContainerdProfileManagement = containerd.AppArmorProfilesManagedAutomatically
	flags.Var((*appArmorProfileManagementFlag)(&o.ContainerdProfileManagement), "containerd-apparmor-management", "how AppArmor profiles for containerd are managed (valid values: auto, host, none)")
	flags.StringVar(&o.ContainerdProfileName, "containerd-apparmor-profile", "k0s-containerd", "name of the AppArmor profile that confines containerd, unless management is none")
}

// Converts the options into the containerd component's configuration.
func (o *AppArmorFlags) ContainerdConfig() containerd.AppArmorConfig {
	return containerd.AppArmorConfig{
		ProfileManagement: o.ContainerdProfileManagement,
		ProfileName:       o.ContainerdProfileName,
	}
}

type appArmorProfileManagementFlag containerd.AppArmorProfileManagement

var _ pflag.Value = (*appArmorProfileManagementFlag)(nil)

func (f *appArmorProfileManagementFlag) Type() string { return "string" }

func (f *appArmorProfileManagementFlag) String() string {
	switch containerd.AppArmorProfileManagement(*f) {
	case containerd.AppArmorProfilesManagedAutomatically:
		return "auto"
	case containerd.AppArmorProfilesManagedOnHost:
		return "host"
	case containerd.NoAppArmorProfileManagement:
		return "none"
	default:
		return strconv.Itoa(int(*f))
	}
}

func (f *appArmorProfileManagementFlag) Set(value string) error {
	switch value {
	case "auto":
		*f = appArmorProfileManagementFlag(containerd.AppArmorProfilesManagedAutomatically)
	case "host":
		*f = appArmorProfileManagementFlag(containerd.AppArmorProfilesManagedOnHost)
	case "none":
		*f = appArmorProfileManagementFlag(containerd.NoAppArmorProfileManagement)
	default:
		return errors.New("valid values: auto, host, none")
	}
	return nil
}
