// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v1beta1

import "fmt"

// Cleans up a slice of interfaces into slice of actual values
func cleanUpInterfaceArray(in []interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, v := range in {
		result[i] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the map keys to be strings
func cleanUpInterfaceMap(in map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range in {
		result[k] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the value in the map, recurses in case of arrays and maps
func cleanUpMapValue(v interface{}) interface{} {
	// Keep null values as nil to avoid type mismatches
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case []interface{}:
		return cleanUpInterfaceArray(v)
	case map[string]interface{}:
		return cleanUpInterfaceMap(v)
	case string:
		return v
	case int:
		return v
	case bool:
		return v
	case float64:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

// CleanUpGenericMap is a helper to "cleanup" generic yaml parsing where nested maps
// are unmarshalled with type map[interface{}]interface{}
func CleanUpGenericMap(in map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range in {
		result[k] = cleanUpMapValue(v)
	}
	return result
}
