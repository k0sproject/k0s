// SPDX-FileCopyrightText: 2022 k0s authors
// SPDX-License-Identifier: Apache-2.0

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

	for i := range rpcmd.NumField() {
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
