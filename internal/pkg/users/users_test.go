/*
Copyright 2022 k0s authors

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

package users

import (
	"runtime"
	"testing"
)

func TestGetUID(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("No numeric user IDs on Windows")
	}

	uid, err := GetUID("root")
	if err != nil {
		t.Errorf("failed to find uid for root: %v", err)
	}
	if uid != 0 {
		t.Errorf("root uid is not 0. It is %d", uid)
	}

	uid, err = GetUID("some-non-existing-user")
	if err == nil {
		t.Errorf("unexpectedly got uid for some-non-existing-user: %d", uid)
	}
}
