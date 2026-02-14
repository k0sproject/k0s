// SPDX-FileCopyrightText: 2021 k0s authors
// SPDX-License-Identifier: Apache-2.0

package archive

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k0sproject/k0s/internal/pkg/file"

	"github.com/sirupsen/logrus"
)

func sanitizeExtractPath(dstDir string, filePath string) (string, error) {
	dstFile := filepath.Join(dstDir, filePath)
	if !strings.HasPrefix(dstFile, filepath.Clean(dstDir)+string(os.PathSeparator)) {
		return "", fmt.Errorf("%s: illegal file path", filePath)
	}
	return dstFile, nil
}

// Extract the given tar.gz archive to given dst path
func Extract(input io.Reader, dst string) error {
	gzr, err := gzip.NewReader(input)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
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
			if err := file.WriteAtomically(targetPath, header.FileInfo().Mode(), func(file io.Writer) error {
				_, err := io.Copy(file, tarReader)
				return err
			}); err != nil {
				return fmt.Errorf("failed to decompress %s from archive: %w", header.Name, err)
			}

		default:
			logrus.Warnf("unknown type %b for archive file %s", header.Typeflag, header.Name)
		}
	}

	return nil
}
