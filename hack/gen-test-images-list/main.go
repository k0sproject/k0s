// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	imageutils "k8s.io/kubernetes/test/utils/image"
)

func main() {
	for _, id := range []imageutils.ImageID{
		imageutils.Agnhost,
		imageutils.JessieDnsutils,
		imageutils.Nginx,
		imageutils.Pause,
	} {
		if _, err := fmt.Println(imageutils.GetE2EImage(id)); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
}
