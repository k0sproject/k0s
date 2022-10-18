// Copyright 2022 k0s authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-exec/tfexec"
)

type TerraformOutputMap map[string]tfexec.OutputMeta

// TerraformOutput is a generic helper for unmarshalling JSON values from
// a terraform output map.
func TerraformOutput[VT any](outputValue *VT, output TerraformOutputMap, key string) error {
	value, found := output[key]
	if !found {
		return fmt.Errorf("unable to find key '%s' in terraform output", key)
	}

	if err := json.Unmarshal(value.Value, outputValue); err != nil {
		return fmt.Errorf("unable to unmarshal value for key '%s' in terraform output", key)
	}

	return nil
}
