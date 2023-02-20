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

package stringmap

import "fmt"

// StringMap defines map like arguments that can be "evaluated" into args=value pairs
type StringMap map[string]string

// Merge merges the input from one map with an existing map, so that we can override entries entry in the existing map
func Merge(inputMap StringMap, existingMap StringMap) StringMap {
	newMap := StringMap{}
	newMap.Merge(existingMap)
	newMap.Merge(inputMap)
	return newMap
}

// ToArgs maps the data into cmd arguments like foo=bar baz=baf
func (m StringMap) ToArgs() []string {
	args := make([]string, len(m))
	idx := 0
	for k, v := range m {
		args[idx] = fmt.Sprintf("%s=%s", k, v)
		idx++
	}
	return args
}

func (m StringMap) ToDashedArgs() []string {
	args := make([]string, len(m))
	idx := 0
	for k, v := range m {
		args[idx] = fmt.Sprintf("--%s=%s", k, v)
		idx++
	}
	return args
}

// Merge merges two maps together
func (m StringMap) Merge(other StringMap) {
	if len(other) > 0 {
		for k, v := range other {
			m[k] = v
		}
	}
}

func (m StringMap) Equals(other StringMap) bool {
	if m == nil && other == nil {
		return true
	}
	if len(m) != len(other) {
		return false
	}
	for k, v := range m {
		val, ok := other[k]
		if !ok || val != v {
			return false
		}
	}
	return true
}
