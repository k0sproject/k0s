// Copyright 2021 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"reflect"
)

const (
	TagAutopilot = "autopilot"
)

// Marshal converts a signalling object to a map, including any nested structs
// that belong to the value. Only fields that have the `autopilot` tag are
// considered for marshalling.
func Marshal(m map[string]string, value interface{}) {
	fields := reflect.TypeOf(value)
	values := reflect.ValueOf(value)

	for i := 0; i < fields.NumField(); i++ {
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

type UnmarshalFieldTypeCollector func() reflect.Type
type UnmarshalFieldValueCollector func() reflect.Value

// Unmarshal uses reflection semantics to turn the marshalled map of values
// back into a structure of unknown type. By relying on two reflection helper
// functions, the reflect types + values can be specialized by the caller,
// allowing this to be reused for all types.
//
// This all assumes that the types can be assigned via string.
func Unmarshal(m map[string]string, uftc UnmarshalFieldTypeCollector, ufvc UnmarshalFieldValueCollector) {
	if m == nil {
		return
	}

	fields := uftc()
	values := ufvc()

	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		value := values.Field(i)

		if value.Kind() == reflect.Struct {
			Unmarshal(
				m,
				func() reflect.Type {
					return field.Type
				},
				func() reflect.Value {
					return value
				},
			)
		} else {
			if fieldTagValue, ok := field.Tag.Lookup(TagAutopilot); ok {
				if mapValue, ok := m[fieldTagValue]; ok {
					value.SetString(mapValue)
				}
			}
		}
	}
}
