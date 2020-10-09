package v1beta1

import (
	"fmt"
)

// WorkerProfiles profiles collection
type WorkerProfiles map[string]WorkerProfile

// Validate validates all profiles
func (wps WorkerProfiles) Validate() []error {
	var errors []error
	for _, p := range wps {
		if err := p.Validate(); err != nil {
			errors = append(errors, err)
		}
	}
	return errors
}

// WorkerProfile worker profile
type WorkerProfile struct {
	Name   string            `yaml:"name"`
	Values map[string]string `yaml:"values"`
}

var lockedFields = map[string]struct{}{
	"clusterDNS":    {},
	"clusterDomain": {},
	"apiVersion":    {},
	"kind":          {},
}

// Validate validates instance
func (wp *WorkerProfile) Validate() error {
	for field, _ := range wp.Values {
		if _, found := lockedFields[field]; found {
			return fmt.Errorf("field `%s` is prohibited to override in worker profile", field)
		}
	}
	return nil
}
