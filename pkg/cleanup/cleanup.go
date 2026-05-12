// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package cleanup

import (
	"context"
	"errors"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	"github.com/k0sproject/k0s/pkg/config"

	"github.com/sirupsen/logrus"
)

type Config struct {
	cleanupSteps []Step
}

func NewConfig(debug bool, k0sVars *config.CfgVars, systemUsers *k0sv1beta1.SystemUser, criSocketFlag string) (*Config, error) {
	steps, err := buildSteps(debug, k0sVars, systemUsers, criSocketFlag)
	if err != nil {
		return nil, err
	}
	return &Config{cleanupSteps: steps}, nil
}

func (c *Config) Cleanup(ctx context.Context) error {
	var errs []error

	for _, step := range c.cleanupSteps {
		logrus.Info("* ", step.Name())
		if err := step.Run(ctx); err != nil {
			logrus.Debug(err)
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during clean-up: %w", errors.Join(errs...))
	}
	return nil
}

// Step interface is used to implement cleanup steps
type Step interface {
	// Run implements specific cleanup operations
	Run(ctx context.Context) error
	// Name returns name of the step for convenience
	Name() string
}
