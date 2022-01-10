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
package stringslice

import (
	"reflect"
	"sort"
)

// Contains check whether the given string slice contains the other string
func Contains(strSlice []string, str string) bool {
	for _, s := range strSlice {
		if s == str {
			return true
		}
	}

	return false
}

// IsEqual returns true if an array of strings is equal, regardless of order
func IsEqual(a1 []string, a2 []string) bool {
	sort.Strings(a1)
	sort.Strings(a2)
	if len(a1) == len(a2) {
		return reflect.DeepEqual(a1, a2)
	}
	return false
}

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
