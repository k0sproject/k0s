// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package token

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"

	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// DecodeJoinToken decodes an token string that is encoded with JoinEncode
func DecodeJoinToken(token string) ([]byte, error) {
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
	closeErr := gz.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	return buf.Bytes(), nil
}

func GetTokenType(clientCfg *clientcmdapi.Config) string {
	for _, kubeContext := range clientCfg.Contexts {
		return kubeContext.AuthInfo
	}

	return ""
}
