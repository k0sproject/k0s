//go:build !windows

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package log

func installBackend() (Backend, ShutdownLoggingFunc) { return nil, func() {} }
