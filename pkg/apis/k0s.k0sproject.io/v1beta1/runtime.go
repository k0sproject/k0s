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
package v1beta1

import (
	"encoding/base64"
	"fmt"
)

// RuntimeSpec defines the runtime related config options
type RuntimeSpec struct {
	// Registries defines private registry related config options
	Registries []*RegistryConfig `json:"registries"`
}

// DefaultRuntimeSpec creates RuntimeSpec with sane defaults
func DefaultRuntimeSpec() *RuntimeSpec {
	return &RuntimeSpec{
		Registries: []*RegistryConfig{
			{
				Name:         "registry1",
				Capabilities: []string{"pull", "resolve"},
				SkipVerify:   true,
				Server:       "registry1",
				CACert:       "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUM5ekNDQWQrZ0F3SUJBZ0lKQU5oVmFPMmtxK3htTUEwR0NTcUdTSWIzRFFFQkN3VUFNQkl4RURBT0JnTlYKQkFNTUIzUmxjM1F0WTJFd0hoY05Nakl3TWpBNE1USXlNak0yV2hjTk1qSXdOREE1TVRJeU1qTTJXakFTTVJBdwpEZ1lEVlFRRERBZDBaWE4wTFdOaE1JSUJJakFOQmdrcWhraUc5dzBCQVFFRkFBT0NBUThBTUlJQkNnS0NBUUVBCjVaU3IzR0QvRmRhZEdnakZwNjBPbG51M0NsQ1FQM1BPeXVXWVJuTVF4SHZTRUdiRkpkcTJhNXFuWFdqNzJ0OWEKaHhmRHdkdWk5RHdOd3ZGSUhrZ2thdU1WbjZOQWNiSzFMUlNVYk10RnYzSUY5cUZDZ3p1UnNtUGpmQjdKM2U1egpucXRsd2YyR2MwY3djb081S0JLZ09JSFRNZ1dOTkw5dzl3RGtSWjUyVGtVR1o4MG8wN2wzQjhaQTV0MkJoS2RNClpTY2VianFXNEJkQk54eWMyeEpjZzQ2MnF4QnB2S3Z6NmR3dThQQjNHNEgzTlM1MDBFRGZYVXBVWElBT2NlYlkKeFoyMmxLODJnN21zSFRzL0RYQW0rNk04Qzl5VVhodGFNYnBacWM3WFErTGZacStRd0hXRit6VVRLQ2VCWUd2RwpmTmdGa2JYK0V2V2tOWXV2eFNxQnhRSURBUUFCbzFBd1RqQWRCZ05WSFE0RUZnUVVjVEwyQzR1Rks0VGJhb1pVCnpVa0ZzZ0hveHJrd0h3WURWUjBqQkJnd0ZvQVVjVEwyQzR1Rks0VGJhb1pVelVrRnNnSG94cmt3REFZRFZSMFQKQkFVd0F3RUIvekFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBa3J1ODVNMTlSYTRBSU9HbmttZUk3ZFBsOU5VMgpUYTNiaVFLN0J6WU9qZ2VZUXVxbnpvaHBEM0t4QzFBdEdCZG5SeWtvRUwzWVlxM0liZXB3Zy9MeUpWczNNUDJXCm8rMGZGRFo2RWFNRzNNNEd1SFdzb0tvbHZuRXJkY2QrUnU1VGFoU2J3RjN1Yys0TFh2ZGliOWNXa01nZFQ2VWgKMExKWHJaTVA1K1QwM3RWdldreldmR2NUZXh4NlgrQmYxUnFrRm1BWmhuNUI0RjZlQ2liVW1ON0RMSlQ3QUlTSgptWWFsNWJ3K1IyYzZEUEZaK3FBQUlES0FXSDFCU2VTT2lEWTM3aStCNStsZW8yYVNCMTdDb0UvWUpWaWF4Z1BCCjVlcEZrcU9vV3IwbWx6MzVSZXZPU1A0cWJXSTBHMnRPR3orcmlVUWlMMXhXOFFMOVUxUTUyU29FRFE9PQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0t",
				ClientCert:   "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURBekNDQWV1Z0F3SUJBZ0lKQUxseVU3RFEvVzZCTUEwR0NTcUdTSWIzRFFFQkN3VUFNQkl4RURBT0JnTlYKQkFNTUIzUmxjM1F0WTJFd0hoY05Nakl3TWpBNE1USXlNalV6V2hjTk1qSXdOREE1TVRJeU1qVXpXakFVTVJJdwpFQVlEVlFRRERBbHlaV2RwYzNSeWVURXdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCCkFRRENCdVJkc211YzBXU2NpVWVRM2F6YWJsYXVzNHZQQzVmWFJMZVduV3gxLzl2VHJJRU1YeGZBbGVQU2JQc0UKQU1BbVlZOEV2ajcvOXFjd0JEU2czU2c4eGIzNFBRWmZHU0Y0RHluR0VQN1BvTjZTVWJQc2c1aHdxSXA0RFhhOQovL0JEcVpuVDlGNExpZk01NDhPZmZDQ2ZrSXk4OGhaN01BcjhrcWtId0lGcDVhZ2ttN3A0WkZRL290L1dqWGxQCnZUSW13OGdLVVpvTHBPa3pBUEJsSURkekVvYmMxZkM5Q0I2TWduT0h0OFcwVXo3amxoVFkweHVnUXN2S2JrbVQKbGl6VTdlcnM0VXozL0JwMVdXcklCTTdNMjRjV0N4cWRIbGJBQ0FaYTB1ditoUmRiYURibHg0QWpRUkptMmRFRQpISERJMGtGeWxrMktxUVV6ajV3WVlIOHBBZ01CQUFHaldqQllNQWtHQTFVZEV3UUNNQUF3Q3dZRFZSMFBCQVFECkFnWGdNQjBHQTFVZEpRUVdNQlFHQ0NzR0FRVUZCd01DQmdnckJnRUZCUWNEQVRBZkJnTlZIUkVFR0RBV2dnbHkKWldkcGMzUnllVEdDQ1hKbFoybHpkSEo1TVRBTkJna3Foa2lHOXcwQkFRc0ZBQU9DQVFFQXU5R3RSaTFoSnpSbQpDUWFMQkw5dklWNTd1TDBJcXpHUTNpZDhHVXdyQ0h2bEZUbUc2QWFTRmlQRmRRT09OWWV5NWRoZURjZEtnbEI4Ckt1dllqOVJ0YXNZZmM1dlBPM0ErY0Y5UzdiSUNGaUZHM1BEZFdrLyt0ZjdhZFZXdWNSR0wzQ2JaU24vK1Y0N3oKNXIxbkVkMzF5cHhEM0swZUhLZTBwMDNtaE1nK2ZWR3A1TXlzdzFYdUh3K01EZmpjUzladk5XdkdTVFkzaGx0UQorcTFZTlJ6Wm1LaEltQkxmU0MxbWRQdnFQbk5RWnEzNmJtZG5lS0E0OEQ2S2JLRC9pT1dyempTMzllTFZwVGFBClA1ZHd0R2dUSVFsaTltYS8rSUFmQTI4Sk9NYXdGZUlzSU84N1doWkNwYjNaY2JITmxTV0VLNWxWeUpSeEVuYlYKSjc2OGxIZ2tCZz09Ci0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=",
				ClientKey:    "LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBd2dia1hiSnJuTkZrbklsSGtOMnMybTVXcnJPTHp3dVgxMFMzbHAxc2RmL2IwNnlCCkRGOFh3SlhqMG16N0JBREFKbUdQQkw0Ky8vYW5NQVEwb04wb1BNVzkrRDBHWHhraGVBOHB4aEQrejZEZWtsR3oKN0lPWWNLaUtlQTEydmYvd1E2bVowL1JlQzRuek9lUERuM3dnbjVDTXZQSVdlekFLL0pLcEI4Q0JhZVdvSkp1NgplR1JVUDZMZjFvMTVUNzB5SnNQSUNsR2FDNlRwTXdEd1pTQTNjeEtHM05Yd3ZRZ2VqSUp6aDdmRnRGTSs0NVlVCjJOTWJvRUxMeW01Sms1WXMxTzNxN09GTTkvd2FkVmxxeUFUT3pOdUhGZ3NhblI1V3dBZ0dXdExyL29VWFcyZzIKNWNlQUkwRVNadG5SQkJ4d3lOSkJjcFpOaXFrRk00K2NHR0IvS1FJREFRQUJBb0lCQVFDbExSamNhemdSbUhEKwpraC9Ldyt5VFI3dWpubFkzUExkWEc3anZEN1YxL3dzMWVIV2tBcEJGODFTdm52ZFN3UkRUbTlvVlA2Q0NaNGlNCjZBZWxxcURHbTlETnM2WG83NHYrbVdvR3BCRkkwTHFwOWNRbVpTRXhSMG9hU2R2OGhCWVdoQnZneFBnSytyV0YKWXREMnhNVkJFZDIrUEpuRzVXOTA5YVhRWTZISWM2NDFOeTQrSk0xQVdSVWxqcG1sSGJGVnl4R25xYmNpTk41QQpFYzVuYzhaRWdNd1h3UzhjeXN0ZXI2cHQvOVNteCtKK2REZEoyQjVmN204TGhucW9JQW1zUTJ4NGE2YXVKeHBPCnU3RmFqVWRhYnhJVXJ6TmZKQUUzZjRBMWhRT1hjMlhKd1Q5Z2Jva3B2YU5kVFBHaW0yUXY0TXQ3ekZlSXF4NVYKc2lmVW0zY0JBb0dCQU96ZjBTY2haVUJuRUZqQ2podVJqQWNXR3hTckdPMFFFOElKYVltYjd2dVcwM3FBZDdBSwp3TXFOckl5UW5ZaEJZbHJzY2t5Q3JBb24welNVRVdZblhiYmJJV2R1MUhoYy9rbVdNVUdBemRrU1hoQlh0dHg2CmFyY09IQm5JTGdPWWxQYWo5U3ZjbWx0ZEY4cUFkcDhXOVJOeHZRVVF1b0VJTFZCZ0o2dFlxM2t0QW9HQkFOR3gKYSsxU3BxSUhQRkFGejZaVmQ4T3VGQkxNdElpRGpRb2tBdlpZSDFTMTBVb1FaVVRqNzN4VW01R0ZNQnNQWjhmMwpDUnM3U1FXZEZid1lQTFE1RXdLNjkrK0JkcUJjcS8zKzJFamtRYjVEckp6VU0zWmhKUG5MNkVxWVdMdnNzbVhqCkdXOHF2S0NEd3l4UnBXdTAyNUdQY1hlU0lVN2Zha1pVNktZemQrTnRBb0dBSmFBZnJ1R0ZIY2ZCTnZnZ2JveWQKKzNvdGJ3a0dlcEYxTWZzZ3duVDhid1kwTFY4K283M3hoYnNmVDJ5aE9VVjVoQXZPMUF4bG0yOWNBeHdKNzNvTgpUc0JiKy93RXorR2xtcmE2dURibmU3V0pMM3RmVm9JemRVUk9mbUhudlRaOVl2Z0VONlZnOTJaQUl6Qm9wemlVClVUUmQyL0llVGVTb25mM0lEMVdVVnprQ2dZQUdqTE1oUzRhVzR3RDRRdVkwZk5EcjFNRWR3VFVXV24yS1JvdXQKSkIxK2FOdHJvODExOUdTampvVDVhNTZRQ2RBbEI4dEtCWFVIYnR1aDcyUGVBVFpkekhjNERPUW1xQjViSloyZAowVHRZZFFhc00xaVVKdjZmcXNYTHBxeUcyaUxNV2VhT2VWaEE3enltWXJwMi9jUXA3TUFQaXdudFM3OG5DVG5uCkR3NThsUUtCZ0gvaFhKMWEvM056aHo1NWFEOEJqMVBTZGNWWVU4d3VlSUs5QlU5U003elovc0Vrc3R3UlM3U1kKSUQxeWtTYkU0cklVSDRDZDNpWHFkeVJIWmxTM2NQNElmOElDbFFob1BuUHJBTW44WE9pcXBsRHkvVThTOXdnMwpDOHJKV05tM0U2YnBFRnRhaHVkYWlYcGNHSzZwOE9FQ0V4MHFvdzFxN2VJWGc5MXMrdTIrCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0t",
			},
		},
	}
}

// Validate validates runtime specs correctness
func (r *RuntimeSpec) Validate() []error {
	var errors []error

	if r.Registries != nil {
		for _, registry := range r.Registries {
			errors = append(errors, validateOptionalProperties(registry)...)
		}
	}

	return errors
}

// RegistryConfig defines the Registry related config options
type RegistryConfig struct {
	// Name of registry
	Name string `json:"name"`

	// Capabilities
	Capabilities []string `json:"capabilities"`

	// Skip the registry certificate verification
	SkipVerify bool `json:"skip_verify"`

	// Server address of registry
	Server string `json:"server"`

	// CA certificate of registry
	CACert string `json:"ca"`

	// Client certificate of registry
	ClientCert string `json:"client"`

	// Client key of registry
	ClientKey string `json:"key"`
}

func validateOptionalProperties(r *RegistryConfig) []error {
	var errors []error

	if r != nil {
		if r.Name == "" {
			errors = append(errors, fmt.Errorf("spec.runtime.registries.name cannot be empty"))
		}

		if r.Server == "" {
			errors = append(errors, fmt.Errorf("spec.runtime.registries.server cannot be empty"))
		}

		if !IsBase64(r.CACert) {
			errors = append(errors, fmt.Errorf("spec.runtime.registries.ca cannot be empty"))
		}

		if !IsBase64(r.ClientCert) {
			errors = append(errors, fmt.Errorf("spec.runtime.registries.client cannot be empty"))
		}

		if !IsBase64(r.ClientKey) {
			errors = append(errors, fmt.Errorf("spec.runtime.registries.key cannot be empty"))
		}
	}

	return errors
}

func IsBase64(s string) bool {
	if s == "" {
		return false
	}
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}
