/*
Copyright 2021 k0s authors

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

var _ Validateable = (*ClusterExtensions)(nil)

// ClusterExtensions specifies cluster extensions
type ClusterExtensions struct {
	Helm *HelmExtensions `json:"helm"`
}

// HelmExtensions specifies settings for cluster helm based extensions
type HelmExtensions struct {
	Repositories []Repository `json:"repositories"`
	Charts       []Chart      `json:"charts"`
}

// Chart single helm addon
type Chart struct {
	Name      string `json:"name"`
	ChartName string `json:"chartname"`
	Version   string `json:"version"`
	Values    string `json:"values"`
	TargetNS  string `json:"namespace"`
}

// Repository describes single repository entry. Fields map to the CLI flags for the "helm add" command
type Repository struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	CAFile   string `json:"caFile"`
	CertFile string `json:"certFile"`
	Insecure bool   `json:"insecure"`
	KeyFile  string `json:"keyfile"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Validate stub for Validateable interface
func (e *ClusterExtensions) Validate() []error {
	return nil
}
