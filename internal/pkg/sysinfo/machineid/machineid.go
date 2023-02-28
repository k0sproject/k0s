/*
Copyright 2020 k0s authors

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

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/denisbrodbeck/machineid"
)

type MachineID struct {
	id, source string
}

func (id *MachineID) ID() string {
	if id == nil {
		return ""
	}

	return id.id
}

func (id *MachineID) Source() string {
	if id == nil {
		return ""
	}

	return id.source
}

func (id *MachineID) String() string {
	return fmt.Sprintf("%q (from %s)", id.id, id.source)
}

// Generate returns protected id for the current machine
func Generate() (*MachineID, error) {
	id, err := machineid.ProtectedID("k0sproject-k0s")
	if err != nil {
		return fromHostname()
	}
	return &MachineID{id, "machine"}, err
}

// fromHostname generates a machine id hash from hostname
func fromHostname() (*MachineID, error) {
	name, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}
	sum := md5.Sum([]byte(name))
	return &MachineID{hex.EncodeToString(sum[:]), "hostname"}, nil
}
