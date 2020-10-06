package token

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
)

// JoinDecode decodes an token string that is encoded with JoinEncode
func JoinDecode(token string) ([]byte, error) {
	gzData, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	gz, err := gzip.NewReader(bytes.NewBuffer(gzData))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	_, err = io.Copy(&buf, gz)
	gzErr := gz.Close()
	if err != nil {
		return nil, err
	}
	if gzErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
