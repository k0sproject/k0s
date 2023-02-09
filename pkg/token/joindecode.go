/*
Copyright 2020 k0s authors

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
	gzErr := gz.Close()
	if err != nil {
		return nil, err
	}
	if gzErr != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func GetTokenType(clientCfg *clientcmdapi.Config) string {
	for _, kubeContext := range clientCfg.Contexts {
		return kubeContext.AuthInfo
	}

	return ""
}
