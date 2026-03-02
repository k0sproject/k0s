// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package sysservice

import (
	"context"
	"errors"
)

type Spec struct {
	Name        string
	DisplayName string
	Description string

	Exec string
	Args []string
	Env  []string // ["K=V", ...]

	WorkingDir string
	User       string

	Autostart bool
}

type Status int

const (
	StatusUnknown Status = iota
	StatusNotInstalled
	StatusStopped
	StatusRunning
)

type Service interface {
	Kind() string

	Install(ctx context.Context) error
	Uninstall(ctx context.Context) error

	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error

	Status(ctx context.Context) (Status, error)
}

var ErrInvalidSpec = errors.New("invalid spec")

func New(spec Spec, opts ...Option) (Service, error) {
	if spec.Name == "" || spec.Exec == "" {
		return nil, ErrInvalidSpec
	}

	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	kind := cfg.kind
	if kind == "" {
		var err error
		kind, err = Detect(cfg.root)
		if err != nil {
			return nil, err
		}
	}

	switch kind {
	case "openrc":
		return newOpenRC(spec, cfg), nil
		/*
			case "systemd":
				return newSystemd(spec, cfg), nil
			case "windows":
				return newWindows(spec, cfg), nil
		*/
	default:
		return nil, errors.New("unsupported kind: " + kind)
	}
}
