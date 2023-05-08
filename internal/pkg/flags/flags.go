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

package flags

import (
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/stringmap"
)

// Split splits arbitrary set of flags into StringMap struct
func Split(input string) stringmap.StringMap {
	mArgs := stringmap.StringMap{}
	args := strings.Fields(input)
	for _, a := range args {
		av := strings.SplitN(a, "=", 2)
		if len(av) < 1 {
			continue
		}
		if len(av) == 1 {
			mArgs[av[0]] = ""
		} else {
			mArgs[av[0]] = av[1]
		}
	}

	return mArgs
}
