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
package controller

import (
	"io/ioutil"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/k0sproject/k0s/internal/util"
	"github.com/k0sproject/k0s/pkg/apis/v1beta1"
	"github.com/k0sproject/k0s/pkg/constant"
	"github.com/k0sproject/k0s/pkg/token"
)

// CASyncer is the Component implementation to sync CAs between multiple controllers
type CASyncer struct {
	JoinClient *token.JoinClient
	K0sVars    constant.CfgVars
}

// Init initializes the CASyncer component
func (c *CASyncer) Init() error {
	caData, err := c.JoinClient.GetCA()
	if err != nil {
		return errors.Wrapf(err, "failed to sync CA")
	}
	// Dump certs into files
	return writeCerts(caData, c.K0sVars)
}

// Run does nothing, there's nothing running constantly
func (c *CASyncer) Run() error {
	return nil
}

// Stop does nothing, there's nothing running constantly
func (c *CASyncer) Stop() error {
	return nil
}

func writeCerts(caData v1beta1.CaResponse, k0sVars constant.CfgVars) error {
	keyFile := filepath.Join(k0sVars.CertRootDir, "ca.key")
	certFile := filepath.Join(k0sVars.CertRootDir, "ca.crt")

	if util.FileExists(keyFile) && util.FileExists(certFile) {
		logrus.Warnf("ca certs already exists, not gonna overwrite. If you wish to re-sync them, delete the existing ones.")
		return nil
	}

	err := ioutil.WriteFile(keyFile, caData.Key, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(certFile, caData.Cert, constant.CertMode)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(k0sVars.CertRootDir, "sa.key"), caData.SAKey, constant.CertSecureMode)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(filepath.Join(k0sVars.CertRootDir, "sa.pub"), caData.SAPub, constant.CertMode)
	if err != nil {
		return err
	}

	return nil
}

// Health-check interface
func (c *CASyncer) Healthy() error { return nil }
