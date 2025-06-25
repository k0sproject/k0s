// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

// JoinEncode compresses and base64 encodes a join token
func JoinEncode(in io.Reader) (string, error) {
	var outBuf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&outBuf, gzip.BestCompression)
	if err != nil {
		return "", err
	}

	_, err = io.Copy(gz, in)
	gzErr := gz.Close()
	if err != nil {
		return "", err
	}
	if gzErr != nil {
		return "", gzErr
	}

	return base64.StdEncoding.EncodeToString(outBuf.Bytes()), nil
}
