// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package signaling

// Validator allows objects to be validated.
type Validator interface {
	Validate() error
}
