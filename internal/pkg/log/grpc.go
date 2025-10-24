// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/grpclog"
)

func newGRPCLogger(log logrus.FieldLogger) grpclog.LoggerV2 {
	// https://github.com/grpc/grpc-go/blob/v1.76.0/grpclog/loggerv2.go#L65-L79

	levelValue := os.Getenv("GRPC_GO_LOG_SEVERITY_LEVEL")
	var level logrus.Level
	switch levelValue {
	case "", "ERROR", "error": // If env is unset, set level to ERROR.
		level = logrus.ErrorLevel
	case "WARNING", "warning":
		level = logrus.WarnLevel
	case "INFO", "info":
		level = logrus.InfoLevel
	}

	var v int
	vLevel := os.Getenv("GRPC_GO_LOG_VERBOSITY_LEVEL")
	if vl, err := strconv.Atoi(vLevel); err == nil {
		v = vl
	}

	return &logrusGRPCLogger{log, level, v}
}

type logrusGRPCLogger struct {
	log       logrus.FieldLogger
	level     logrus.Level
	verbosity int
}

func (l *logrusGRPCLogger) Error(args ...any) {
	if l.level >= logrus.ErrorLevel {
		l.log.Error(args...)
	}
}

func (l *logrusGRPCLogger) Errorf(format string, args ...any) {
	if l.level >= logrus.ErrorLevel {
		l.log.Errorf(format, args...)
	}
}

func (l *logrusGRPCLogger) Errorln(args ...any) {
	if l.level >= logrus.ErrorLevel {
		l.log.Errorln(args...)
	}
}

func (l *logrusGRPCLogger) Fatal(args ...any) {
	if l.level >= logrus.FatalLevel {
		l.log.Fatal(args...)
	}
}

func (l *logrusGRPCLogger) Fatalf(format string, args ...any) {
	if l.level >= logrus.FatalLevel {
		l.log.Fatalf(format, args...)
	}
}

func (l *logrusGRPCLogger) Fatalln(args ...any) {
	if l.level >= logrus.FatalLevel {
		l.log.Fatalln(args...)
	}
}

func (l *logrusGRPCLogger) Info(args ...any) {
	if l.level >= logrus.InfoLevel {
		l.log.Info(args...)
	}
}

func (l *logrusGRPCLogger) Infof(format string, args ...any) {
	if l.level >= logrus.InfoLevel {
		l.log.Infof(format, args...)
	}
}

func (l *logrusGRPCLogger) Infoln(args ...any) {
	if l.level >= logrus.InfoLevel {
		l.log.Infoln(args...)
	}
}

func (l *logrusGRPCLogger) Warning(args ...any) {
	if l.level >= logrus.WarnLevel {
		l.log.Warning(args...)
	}
}

func (l *logrusGRPCLogger) Warningf(format string, args ...any) {
	if l.level >= logrus.WarnLevel {
		l.log.Warningf(format, args...)
	}
}

func (l *logrusGRPCLogger) Warningln(args ...any) {
	if l.level >= logrus.WarnLevel {
		l.log.Warningln(args...)
	}
}

func (l *logrusGRPCLogger) V(level int) bool {
	return l.verbosity >= level
}
