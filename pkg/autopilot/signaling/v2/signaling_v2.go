// Copyright 2021 k0s authors
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

package v2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/k0sproject/k0s/pkg/autopilot/signaling"
)

const (
	Version = "v2"
)

// Signal is a struct representation of `autopilot` annotations. The details of the
// `autopilot` commands are JSON-encoded in the provided data field.
type Signal struct {
	Version string `autopilot:"k0sproject.io/autopilot-signal-version" validate:"eq=v2,required,alphanum"`
	Data    string `autopilot:"k0sproject.io/autopilot-signal-data" validate:"required"`
}

var _ signaling.Validator = (*Signal)(nil)

// Validate ensures that all of the `Signal` values adhere to validation requirements.
func (e Signal) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	return validate.Struct(e)
}

// Status maintains a status along with the time of the status change.
type Status struct {
	Status    string `json:"status" validate:"required"`
	Timestamp string `json:"timestamp" validate:"required"`
}

// NewStatus creates a new status with a timestamp.
func NewStatus(status string) *Status {
	return &Status{Status: status, Timestamp: time.Now().Format(time.RFC3339)}
}

// SignalData provides all of the details of the requested `autopilot` operation,
// as well as its current status.
type SignalData struct {
	PlanID  string  `json:"planId" validate:"required"`
	Created string  `json:"created" validate:"required"`
	Command Command `json:"command" validate:"required"`
	Status  *Status `json:"status,omitempty"`
}

var _ signaling.Validator = (*SignalData)(nil)

// Validate ensures that all of the `SignalData` values adhere to validation requirements.
func (s SignalData) Validate() error {
	validate := validator.New(validator.WithRequiredStructEnabled())
	validate.RegisterStructValidation(validateCommand, Command{})

	return validate.Struct(s)
}

// Marshal will serialize the `Signaling` instance into the provided map. This includes
// the conversion to an `Envelope`.
func (s SignalData) Marshal(m map[string]string) error {
	data, err := json.Marshal(&s)
	if err != nil {
		return fmt.Errorf("unable to JSON marshal signaling data: %w", err)
	}

	Marshal(m, Signal{
		Version: Version,
		Data:    string(data),
	})

	return nil
}

// Unmarshal will extract the known `autopilot` annotation values from the provided
// map, and deserialize them into a `SignalData` instance. Validation occurs at both
// the `Envelope` and `SignalData` unmarshaling phases.
func (s *SignalData) Unmarshal(m map[string]string) error {
	e := &Signal{}

	Unmarshal(
		m,
		func() reflect.Type {
			return reflect.TypeOf(*e)
		},
		func() reflect.Value {
			return reflect.ValueOf(e).Elem()
		},
	)

	if err := e.Validate(); err != nil {
		return fmt.Errorf("signaling envelope validation failure: %w", err)
	}

	stmp := SignalData{}
	if err := json.Unmarshal([]byte(e.Data), &stmp); err != nil {
		return fmt.Errorf("signaling data unmarshal failure: %w", err)
	}

	if err := stmp.Validate(); err != nil {
		return fmt.Errorf("signaling data validation failure: %w", err)
	}

	*s = stmp

	return nil
}

// IsSignalingPresent determines if signaling annotations are present in
// the provided map
func IsSignalingPresent(m map[string]string) bool {
	_, versionFound := m["k0sproject.io/autopilot-signal-version"]
	_, dataFound := m["k0sproject.io/autopilot-signal-data"]

	return versionFound && dataFound
}

// Command contains all of the at-most-one commands that can be used to control
// an `autopilot` operation. Currently only `update` is supported.
type Command struct {
	ID           *int                 `json:"id" validate:"required"`
	K0sUpdate    *CommandK0sUpdate    `json:"k0supdate,omitempty"`
	AirgapUpdate *CommandAirgapUpdate `json:"airgapupdate,omitempty"`
}

// CommandK0sUpdate describes what an update to `k0s` is.
type CommandK0sUpdate struct {
	URL         string `json:"url" validate:"required,url"`
	Version     string `json:"version" validate:"required"`
	Sha256      string `json:"sha256,omitempty"`
	ForceUpdate bool   `json:"forceupdate,omitempty"`
}

// CommandAirgapUpdate describes what an update to `airgap` is.
type CommandAirgapUpdate struct {
	URL     string `json:"url" validate:"required,url"`
	Version string `json:"version" validate:"required"`
	Sha256  string `json:"sha256,omitempty"`
}

// validateCommand ensures that a `Command` contains at-most-one of
// the following fields: `K0sUpdate`, `AirgapUpdate`.
func validateCommand(sl validator.StructLevel) {
	cui := sl.Current().Interface().(Command)

	// Provide at-most-one semantics, ensuring that only one field is defined.
	if (cui.K0sUpdate == nil && cui.AirgapUpdate == nil) || (cui.K0sUpdate != nil && cui.AirgapUpdate != nil) {
		sl.ReportError(reflect.ValueOf(cui.K0sUpdate), "K0sUpdate", "k0supdate", "atmostone", "")
		sl.ReportError(reflect.ValueOf(cui.AirgapUpdate), "AirgapUpdate", "airgapupdate", "atmostone", "")
	}
}
