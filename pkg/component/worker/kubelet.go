/*
Copyright 2020 Mirantis, Inc.

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
package worker

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func splitRuntimeConfig(rtConfig string) (string, string, error) {
	runtimeConfig := strings.SplitN(rtConfig, ":", 2)
	if len(runtimeConfig) != 2 {
		return "", "", fmt.Errorf("cannot parse CRI socket path")
	}
	runtimeType := runtimeConfig[0]
	runtimeSocket := runtimeConfig[1]
	if runtimeType != "docker" && runtimeType != "remote" {
		return "", "", fmt.Errorf("unknown runtime type %s, must be either of remote or docker", runtimeType)
	}

	return runtimeType, runtimeSocket, nil
}

const awsMetaInformationURI = "http://169.254.169.254/latest/meta-data/local-hostname"

func getNodeName() (string, error) {
	req, err := http.NewRequest("GET", awsMetaInformationURI, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return os.Hostname()
	}
	defer resp.Body.Close()
	h, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("can't read aws hostname: %v", err)
	}
	return string(h), nil
}
