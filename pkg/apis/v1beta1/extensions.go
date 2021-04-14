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
	Helm *HelmExtensions `yaml:"helm"`
}

// HelmExtensions specifies settings for cluster helm based extensions
type HelmExtensions struct {
	Repositories []Repository `yaml:"repositories"`
	Charts       []Chart      `yaml:"charts"`
}

// Chart single helm addon
type Chart struct {
	Name      string `yaml:"name"`
	ChartName string `yaml:"chartname"`
	Version   string `yaml:"version"`
	Values    string `yaml:"values"`
	TargetNS  string `yaml:"namespace"`
}

// Repository describes single repository entry. Fields map to the CLI flags for the "helm add" command
type Repository struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	CAFile   string `yaml:"caFile"`
	CertFile string `yaml:"certFile"`
	Insecure bool   `yaml:"insecure"`
	KeyFile  string `yaml:"keyfile"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// Validate stub for Validateable interface
func (e *ClusterExtensions) Validate() []error {
	return nil
}
