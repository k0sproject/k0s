// Copyright © 2018 Zlatko Čalušić
//
// Use of this source code is governed by a BSD-style license that can be found in the LICENSE file.

// Package cpuid gives Go programs access to CPUID opcode.
package cpuid

// CPUID returns processor identification and feature information.
func CPUID(info *[4]uint32, ax uint32)
