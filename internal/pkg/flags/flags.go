// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
