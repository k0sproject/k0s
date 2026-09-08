// SPDX-FileCopyrightText: 2023 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"log/slog"

	"github.com/bombsimon/logrusr/v4"
	cfssllog "github.com/cloudflare/cfssl/log"
	"github.com/go-logr/logr"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

type Backend any

type ShutdownLoggingFunc func()

func InitLogging() (Backend, ShutdownLoggingFunc) {
	backend, shutdown := installBackend()

	customFormatter := new(logrus.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	logrus.SetFormatter(customFormatter)

	cfssllog.SetLogger((*cfsslAdapter)(logrus.WithField("component", "cfssl")))
	crlog.SetLogger(logrusr.New(logrus.WithField("component", "controller-runtime")))
	grpclog.SetLoggerV2(&grpcAdapter{logrus.WithField("component", "grpc")})

	// Helm uses slog directly in some code paths, ensure a default
	// handler is set to avoid bypassing k0s's logging setup
	handler := logr.ToSlogHandler(logrusr.New(logrus.WithField("component", "helm")))
	slog.SetDefault(slog.New(handler))

	SetWarnLevel()

	return backend, shutdown
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

type grpcAdapter struct{ *logrus.Entry }

func (*grpcAdapter) V(level int) bool { return false }
