// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package io

// An adapter to allow the use of an ordinary function as an [io.Writer].
type WriterFunc func(p []byte) (int, error)

// Write implements [io.Writer].
func (f WriterFunc) Write(p []byte) (int, error) { return f(p) }
