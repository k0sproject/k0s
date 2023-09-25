// Copyright 2023 k0s authors
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

package channels

import (
	"github.com/k0sproject/version"
)

type DownloadURL struct {
	Arch         string `yaml:"arch"`
	OS           string `yaml:"os"`
	K0S          string `yaml:"k0s"`
	K0SSha256    string `yaml:"k0sSha256"`
	AirgapBundle string `yaml:"airgapBundle"`
	AirgapSha256 string `yaml:"airgapSha256"`
}

type Channel struct {
	Channel     string `yaml:"channel"`
	EOLDate     string `yaml:"eolDate"`
	VersionInfo `yaml:",inline"`
}

type VersionInfo struct {
	Version      string        `yaml:"version"`
	DownloadURLs []DownloadURL `yaml:"downloadURLs"`
}

func (v *VersionInfo) IsNewerThan(other string) (bool, error) {
	new, err := version.NewVersion(v.Version)
	if err != nil {
		return false, err
	}
	o, err := version.NewVersion(other)
	if err != nil {
		return false, err
	}
	return new.GreaterThan(o), nil
}
