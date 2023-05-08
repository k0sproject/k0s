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

package stringslice

// Unique returns only the unique items from given input slice
func Unique(input []string) []string {
	m := make(map[string]bool)
	result := make([]string, 0, len(input))
	for _, s := range input {
		if _, ok := m[s]; !ok {
			m[s] = true
			result = append(result, s)
		}
	}
	return result
}
