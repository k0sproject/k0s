// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
