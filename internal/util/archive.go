/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package util

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/sirupsen/logrus"
)

func sanitizeExtractPath(dstDir string, filePath string) (string, error) {
	dstFile := filepath.Join(dstDir, filePath)
	if !strings.HasPrefix(dstFile, filepath.Clean(dstDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: illegal file path", filePath)
	}
	return dstFile, nil
}

// ExtractArchive extracts the given tar.gz archive to given dst path
func ExtractArchive(path, dst string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()

	gzr, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}
		targetPath, err := sanitizeExtractPath(dst, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(targetPath, header.FileInfo().Mode()); err != nil {
				return fmt.Errorf("failed to decompress %s from archive: %w", header.Name, err)
			}
		case tar.TypeReg:
			outFile, err := os.OpenFile(targetPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, header.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("failed to decompress %s from archive: %w", header.Name, err)
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return fmt.Errorf("failed to decompress %s from archive: %w", header.Name, err)
			}

		default:
			logrus.Warnf("unknown type %b for archive file %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
