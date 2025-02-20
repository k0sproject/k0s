// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"github.com/bombsimon/logrusr/v4"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/sirupsen/logrus"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

func InitLogging() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	initBuffer()

	cfssllog.SetLogger((*cfsslAdapter)(logrus.WithField("component", "cfssl")))
	crlog.SetLogger(logrusr.New(logrus.WithField("component", "controller-runtime")))

	SetWarnLevel()
}

func ShutdownLogging() {
	shutdownBuffer()
}

func SetDebugLevel() {
	logrus.SetLevel(logrus.DebugLevel)
	cfssllog.Level = cfssllog.LevelDebug
}

func SetInfoLevel() {
	logrus.SetLevel(logrus.InfoLevel)
	cfssllog.Level = cfssllog.LevelInfo
}

func SetWarnLevel() {
	logrus.SetLevel(logrus.WarnLevel)
	cfssllog.Level = cfssllog.LevelWarning
}
