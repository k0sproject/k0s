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

import "sigs.k8s.io/yaml"

// custom DeepcopyInto func for WorkerProfile
func (in *WorkerProfile) DeepCopyInto(out *WorkerProfile) {
	if in == nil {
		return
	}
	b, err := yaml.Marshal(in.Config)
	if err != nil {
		return
	}
	var config *WorkerConfig

	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return
	}
	out.Config = config
}
