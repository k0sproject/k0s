// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package v2

import (
	"reflect"
)

const (
	TagAutopilot = "autopilot"
)

// Marshal converts a signaling object to a map, including any nested structs
// that belong to the value. Only fields that have the `autopilot` tag are
// considered for marshaling.
func Marshal(m map[string]string, value any) {
	fields := reflect.TypeOf(value)
	values := reflect.ValueOf(value)

	for i := range fields.NumField() {
		field := fields.Field(i)
		value := values.Field(i)

		if value.Kind() == reflect.Struct {
			Marshal(m, value.Interface())
		} else {
			if val, ok := value.Interface().(string); ok {
				if fieldName, ok := field.Tag.Lookup(TagAutopilot); ok {
					m[fieldName] = val
				}
			}
		}
	}
}

// Unmarshal uses reflection semantics to turn the marshaled map of values
// back into a structure of unknown type. By relying on two reflection helper
// functions, the reflect types + values can be specialized by the caller,
// allowing this to be reused for all types.
//
// This all assumes that the types can be assigned via string.
func Unmarshal[T any](m map[string]string, target *T) {
	if m == nil {
		return
	}
	unmarshalReflect(m, reflect.TypeOf(*target), reflect.ValueOf(target).Elem())
}

func unmarshalReflect(m map[string]string, fields reflect.Type, values reflect.Value) {
	for i := range fields.NumField() {
		field := fields.Field(i)
		value := values.Field(i)

		if value.Kind() == reflect.Struct {
			unmarshalReflect(m, field.Type, value)
		} else {
			if fieldTagValue, ok := field.Tag.Lookup(TagAutopilot); ok {
				if mapValue, ok := m[fieldTagValue]; ok {
					value.SetString(mapValue)
				}
			}
		}
	}
}
