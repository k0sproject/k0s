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

package cleanup

import (
	"github.com/k0sproject/k0s/pkg/install"
	"github.com/sirupsen/logrus"
)

type users struct {
	Config *Config
}

// Name returns the name of the step
func (u *users) Name() string {
	return "remove k0s users step:"
}

// Run removes all controller users that are present on the host
func (u *users) Run() error {
	cfg, err := u.Config.k0sVars.NodeConfig()
	if err != nil {
		logrus.Errorf("failed to get cluster setup: %v", err)
	}
	if err := install.DeleteControllerUsers(cfg); err != nil {
		// don't fail, just notify on delete error
		logrus.Warnf("failed to delete controller users: %v", err)
	}
	return nil
}
