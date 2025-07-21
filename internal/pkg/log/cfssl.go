// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/sirupsen/logrus"
)

type cfsslAdapter logrus.Entry

var _ cfssllog.SyslogWriter = (*cfsslAdapter)(nil)

// Debug implements log.SyslogWriter
func (a *cfsslAdapter) Debug(msg string) {
	(*logrus.Entry)(a).Debug(msg)
}

// Info implements log.SyslogWriter
func (a *cfsslAdapter) Info(msg string) {
	(*logrus.Entry)(a).Info(msg)
}

// Warning implements log.SyslogWriter
func (a *cfsslAdapter) Warning(msg string) {
	(*logrus.Entry)(a).Warn(msg)
}

// Err implements log.SyslogWriter
func (a *cfsslAdapter) Err(msg string) {
	(*logrus.Entry)(a).Error(msg)
}

// Crit implements log.SyslogWriter
func (a *cfsslAdapter) Crit(msg string) {
	(*logrus.Entry)(a).Error(msg)
}

// Emerg implements log.SyslogWriter
func (a *cfsslAdapter) Emerg(msg string) {
	(*logrus.Entry)(a).Error(msg)
}
