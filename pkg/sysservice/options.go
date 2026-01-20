// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

type Option func(*config)

type config struct {
	kind   string
	root   string // if set, all files are written under this root (tests use TempDir)
	runner Runner
}

func defaultConfig() config {
	return config{
		runner: ExecRunner{},
	}
}

func WithKind(kind string) Option {
	return func(c *config) { c.kind = kind }
}

// WithRoot prefixes filesystem writes/reads under root.
// Example: root=/tmp/x -> writes /tmp/x/etc/systemd/system/...
func WithRoot(root string) Option {
	return func(c *config) { c.root = root }
}

func WithRunner(r Runner) Option {
	return func(c *config) { c.runner = r }
}
