/*
Copyright 2018 The Kubernetes Authors.

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

package docker

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/weaveworks/footloose/pkg/exec"
)

// Inspect return low-level information on containers
func Inspect(containerNameOrID, format string) ([]string, error) {
	cmd := exec.Command("docker", "inspect",
		"-f", // format
		fmt.Sprintf("'%s'", format),
		containerNameOrID, // ... against the "node" container
	)

	return exec.CombinedOutputLines(cmd)

}

// InspectObject is similar to Inspect but deserializes the JSON output to a struct.
func InspectObject(containerNameOrID, format string, out interface{}) error {
	res, err := Inspect(containerNameOrID, fmt.Sprintf("{{json %s}}", format))
	if err != nil {
		return err
	}
	data := []byte(strings.Trim(res[0], "'"))
	err = json.Unmarshal(data, out)
	if err != nil {
		return err
	}
	return nil
}
