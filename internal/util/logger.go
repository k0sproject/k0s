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
package util

import (
	"os"

	"github.com/sirupsen/logrus"
)

// CLILogger returns a logger with timestamps disabled & colors
// Suitable for log output for CLI Operations
func CLILogger() *logrus.Logger {
	logger := logrus.New()
	textFormatter := new(logrus.TextFormatter)
	textFormatter.DisableTimestamp = true
	textFormatter.DisableLevelTruncation = true
	logger.SetFormatter(textFormatter)

	return logger
}

// K0sLogger is the default logger for k0s log file
func K0sLogger() *logrus.Logger {
	logger := logrus.New()
	logger.SetOutput(os.Stdout)
	logger.SetLevel(logrus.InfoLevel)

	textFormatter := new(logrus.TextFormatter)
	textFormatter.TimestampFormat = "2006-01-02 15:04:05"
	textFormatter.FullTimestamp = true
	textFormatter.DisableLevelTruncation = true
	logger.SetFormatter(textFormatter)
	return logger
}
