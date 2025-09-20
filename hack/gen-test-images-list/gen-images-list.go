//go:build hack

// SPDX-FileCopyrightText: 2025 k0s authors
// SPDX-License-Identifier: Apache-2.0

package gentestimageslist

import (
	"flag"
	"fmt"
	"os"

	imageutils "k8s.io/kubernetes/test/utils/image"
)

func GenImagesList(name string, args ...string) error {
	var outputFile, alpineVersion, kubernetesVersion, sonobuoyVersion string

	flags := flag.NewFlagSet(name, flag.ExitOnError)
	flags.SetOutput(os.Stdout)
	flags.StringVar(&outputFile, "o", "", "Output file for the generated images list")
	flags.StringVar(&alpineVersion, "alpine-version", "", "Alpine version to use")
	flags.StringVar(&kubernetesVersion, "kubernetes-version", "", "Kubernetes version to use")
	flags.StringVar(&sonobuoyVersion, "sonobuoy-version", "", "Sonobuoy version to use")
	err := flags.Parse(args)

	if err != nil || outputFile == "" || alpineVersion == "" || kubernetesVersion == "" || sonobuoyVersion == "" {
		buf := flags.Output()
		if err != nil {
			fmt.Fprintln(buf, "Error:", err)
		}
		fmt.Fprintf(buf, "Usage: %s -o <output-file> -alpine-version <alpine-version> -kubernetes-version <kubernetes-version> -sonobuoy-version <sonobuoy-version>\n", name)

		return err
	}

	images := []string{
		"docker.io/library/nginx:1.29.1-alpine",
		"docker.io/curlimages/curl:8.16.0",
		"docker.io/library/alpine:" + alpineVersion,
		"docker.io/sonobuoy/sonobuoy:v" + sonobuoyVersion,
		"registry.k8s.io/conformance:v" + kubernetesVersion,
		imageutils.GetE2EImage(imageutils.Agnhost),
		imageutils.GetE2EImage(imageutils.JessieDnsutils),
		imageutils.GetE2EImage(imageutils.Nginx),
		imageutils.GetE2EImage(imageutils.Pause),
	}

	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create output file %q: %w", outputFile, err)
	}
	defer f.Close()

	for _, img := range images {
		if _, err := fmt.Fprintln(f, img); err != nil {
			return err
		}
	}
	return nil
}
