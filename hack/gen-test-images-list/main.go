// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/k0sproject/k0s/inttest/common/ociimages"

	imageutils "k8s.io/kubernetes/test/utils/image"
)

func main() {
	images := []string{
		ociimages.Nginx,
	}

	for _, id := range []imageutils.ImageID{
		imageutils.Agnhost,
		imageutils.GlibcDnsTesting,
		imageutils.Nginx,
		imageutils.Pause,
	} {
		images = append(images, imageutils.GetE2EImage(id))
	}

	for _, image := range images {
		if _, err := fmt.Println(image); err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
}
