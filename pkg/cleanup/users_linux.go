// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

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
