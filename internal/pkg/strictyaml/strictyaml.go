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

package strictyaml

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// var fieldNamePattern = regexp.MustCompile("field ([^ ]+)")

// YamlUnmarshalStrictIgnoringFields does UnmarshalStrict but ignores type errors for given fields
func YamlUnmarshalStrictIgnoringFields(in []byte, out interface{}, ignore ...string) (err error) {
	err = yaml.UnmarshalStrict(in, &out)
	if err != nil {
		// parse errors for unknown field errors
		for _, field := range ignore {
			unknownFieldErr := fmt.Sprintf("unknown field \"%s\"", field)
			if strings.Contains(err.Error(), unknownFieldErr) {
				// reset err on unknown masked fields
				err = nil
			}
		}
		// we have some other error
		return err
	}
	return nil
}
