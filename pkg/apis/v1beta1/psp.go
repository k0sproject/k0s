package v1beta1

import "github.com/Mirantis/mke/pkg/constant"

// PodSecurityPolicy defines the config options for setting system level default PSP
type PodSecurityPolicy struct {
	DefaultPolicy string `yaml:"defaultPolicy"`
}

// DefaultPodSecurityPolicy creates new PodSecurityPolicy with sane defaults
func DefaultPodSecurityPolicy() *PodSecurityPolicy {
	return &PodSecurityPolicy{
		DefaultPolicy: constant.DefaultPSP,
	}
}
