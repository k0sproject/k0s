// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"os"
	"time"

	"github.com/k0sproject/k0s/internal/supervised"

	"github.com/bombsimon/logrusr/v4"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/sirupsen/logrus"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

var logFile *os.File

func InitLogging() {
	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	if isService, err := supervised.IsService(); err != nil {
		panic(err)
	} else if isService {
		logFile, err = os.CreateTemp("", fmt.Sprintf("k0s_%d_*.log", time.Now().Unix()))
		if err != nil {
			panic(err)
		}
		logrus.SetOutput(logFile)
	}

	cfssllog.SetLogger((*cfsslAdapter)(logrus.WithField("component", "cfssl")))
	crlog.SetLogger(logrusr.New(logrus.WithField("component", "controller-runtime")))

	SetWarnLevel()
}

func ShutdownLogging() {
	_ = logFile.Close()
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
