// Copyright 2022 k0s authors
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

package core

import (
	"reflect"

	apv1beta2 "github.com/k0sproject/k0s/pkg/apis/autopilot/v1beta2"
)

// planCommandProviderLookup will iterate through all of the fields in a `PlanCommand`, looking
// for the first field that has a non-nil value. Once found, an internal map will be consulted
// to find a handler for this field value.
//
// This is effectively a reflective short-cut to avoid long if/else-if blocks searching for
// non-nil field values. (ie. `if cmd.K0sUpdate != nil { processK0sUpdate() } ...`)
func planCommandProviderLookup(pcpm PlanCommandProviderMap, cmd apv1beta2.PlanCommand) (string, PlanCommandProvider, bool) {
	rpcmd := reflect.Indirect(reflect.ValueOf(cmd))

	for i := 0; i < rpcmd.NumField(); i++ {
		v := rpcmd.Field(i)

		if v.Kind() == reflect.Pointer && !v.IsNil() {
			fieldName := rpcmd.Type().Field(i).Name
			if handler, found := pcpm[fieldName]; found {
				return fieldName, handler, true
			}
		}
	}

	return "", nil, false
}
