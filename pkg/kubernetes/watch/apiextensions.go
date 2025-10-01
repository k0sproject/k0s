// SPDX-FileCopyrightText: 2024 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func CRDs(client Provider[*v1.CustomResourceDefinitionList]) *Watcher[v1.CustomResourceDefinition] {
	return FromClient[*v1.CustomResourceDefinitionList, v1.CustomResourceDefinition](client)
}
