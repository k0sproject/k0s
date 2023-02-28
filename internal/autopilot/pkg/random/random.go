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

package random

import (
	"crypto/rand"
)

var letters = "abcdefghijklmnopqrstuvwxyz0123456789"

// String generates a random string with given length
func String(length int) string {

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Not much we can do on broken system
		panic("random is broken: " + err.Error())
	}

	for i, b := range bytes {
		bytes[i] = letters[b%byte(len(letters))]
	}
	return string(bytes)
}
