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
	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/install"

	"github.com/sirupsen/logrus"
)

type users struct {
	systemUsers *k0sv1beta1.SystemUser
}

// Name returns the name of the step
func (u *users) Name() string {
	return "remove k0s users step:"
}

// Run removes all controller users that are present on the host
func (u *users) Run() error {
	if err := install.DeleteControllerUsers(u.systemUsers); err != nil {
		// don't fail, just notify on delete error
		logrus.Warnf("failed to delete controller users: %v", err)
	}
	return nil
}
