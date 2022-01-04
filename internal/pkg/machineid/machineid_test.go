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
package machineid

import "testing"

func TestMachineIDFromHostname(t *testing.T) {
	id, err := fromHostname()
	if err != nil {
		t.Errorf("fromHostname() unexpctedly returned error")
	} else if len(id) != 32 {
		t.Errorf("len(fromHostname()) = %d, want %d", len(id), 32)
	}

	// test that id does not change
	id2, err := fromHostname()
	if err != nil {
		t.Errorf("fromHostname() unexpectedly returned error")
	} else if id != id2 {
		t.Errorf("fromHostname() = %s, want %s", id2, id)
	}
}
