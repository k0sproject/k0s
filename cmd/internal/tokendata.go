// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"errors"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// EnvVarToken is the environment variable name for the join token
const EnvVarToken = "K0S_TOKEN"

const tokenArgDeprecationWarning = "Passing a join token as a CLI argument is deprecated. Use --token-file or the K0S_TOKEN environment variable instead."

// CheckSingleTokenSource verifies that at most one token source is provided.
// Returns an error if multiple sources are specified.
func CheckSingleTokenSource(tokenArg, tokenFile string) error {
	tokenSources := 0
	if tokenArg != "" {
		tokenSources++
	}
	if tokenFile != "" {
		tokenSources++
	}
	if os.Getenv(EnvVarToken) != "" {
		tokenSources++
	}

	if tokenSources > 1 {
		return fmt.Errorf("you can only pass one token source: either as a CLI argument, via '--token-file [path]', or via the %s environment variable", EnvVarToken)
	}

	return nil
}

// WarnDeprecatedTokenArg logs a deprecation warning when a join token is
// provided as a positional CLI argument.
func WarnDeprecatedTokenArg(tokenArg string) {
	if tokenArg == "" {
		return
	}
	logrus.Warn(tokenArgDeprecationWarning)
}

// GetTokenData resolves the join token from multiple possible sources:
// CLI argument, token file, or K0S_TOKEN environment variable.
// Returns empty string if no token source is available.
func GetTokenData(tokenArg, tokenFile string) (string, error) {
	tokenEnvValue := os.Getenv(EnvVarToken)

	if tokenArg != "" {
		return tokenArg, nil
	}

	if tokenEnvValue != "" {
		return tokenEnvValue, nil
	}

	if tokenFile == "" {
		return "", nil
	}

	var problem string
	data, err := os.ReadFile(tokenFile)
	if errors.Is(err, os.ErrNotExist) {
		problem = "not found"
	} else if err != nil {
		return "", fmt.Errorf("failed to read token file: %w", err)
	} else if len(data) == 0 {
		problem = "is empty"
	}
	if problem != "" {
		return "", fmt.Errorf(`token file "%s" %s`+
			`: obtain a new token via "k0s token create ..." and store it in the file`+
			` or reinstall this node via "k0s install --force ..." or "k0sctl apply --force ..."`,
			tokenFile, problem)
	}
	return string(data), nil
}
