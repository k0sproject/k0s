//go:build noembedbins || (!linux && !windows)

// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package assets

var BinData = map[string]struct{ offset, size, originalSize int64 }{}
var BinDataSize int64
