// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Unstructured(client Provider[*unstructured.UnstructuredList]) *Watcher[unstructured.Unstructured] {
	return FromClient[*unstructured.UnstructuredList, unstructured.Unstructured](client)
}
