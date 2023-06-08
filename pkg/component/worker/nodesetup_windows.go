/*
Copyright 2020 k0s authors

Licensed under the Apache License, Version 2.0 (the &quot;License&quot;);
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an &quot;AS IS&quot; BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package worker

import (
	"context"
	"fmt"
	"os"

	"github.com/k0sproject/k0s/internal/pkg/dir"
	"github.com/k0sproject/k0s/pkg/component/manager"
	"github.com/sirupsen/logrus"
)

type CalicoInstaller struct {
	Token      string
	APIAddress string
	CIDRRange  string
	ClusterDNS string
}

var _ manager.Component = (*CalicoInstaller)(nil)

func (c CalicoInstaller) Init(_ context.Context) error {
	// path := "C:\\bootstrap.ps1"

	// if err := dir.Init("C:\\CalicoWindows", 777); err != nil {
	// 	if os.IsExist(err) {
	// 		logrus.Warn("CalicoWindows already set up")
	// 		return nil
	// 	}
	// 	return fmt.Errorf("can't create CalicoWindows dir: %v", err)
	// }
	// // c:\etc\cni\net.d
	// if err := dir.Init("C:\\etc\\cni\\net.d", 777); err != nil {
	// 	if os.IsExist(err) {
	// 		logrus.Warn("CalicoWindows already set up")
	// 	} else {
	// 		return fmt.Errorf("can't create CalicoWindows dir: %v", err)
	// 	}
	// }
	// // C:\Program Files\containerd\cni\bin
	// if err := dir.Init("C:\\Program Files\\containerd\\cni\\bin", 777); err != nil {
	// 	if os.IsExist(err) {
	// 		logrus.Warn("CalicoWindows already set up")
	// 	} else {
	// 		return fmt.Errorf("can't create CalicoWindows dir: %v", err)
	// 	}
	// }

	for _, path := range []string{
		"C:\\opt\\cni\\bin",
		"C:\\opt\\cni\\conf",
	} {
		if err := dir.Init(path, 777); err != nil {
			if os.IsExist(err) {
				logrus.Warn("CalicoWindows already set up")
			} else {
				return fmt.Errorf("can't create CalicoWindows dir: %v", err)
			}
		}
	}
	return winExecute(installCalicoPowershell)
	// return nil
}

func (c CalicoInstaller) SaveKubeConfig(path string) error {
	return nil
	// tokenBytes, err := token.DecodeJoinToken(c.Token)
	// if err != nil {
	// 	return fmt.Errorf("failed to decode token: %v", err)
	// }
	// clientConfig, err := clientcmd.NewClientConfigFromBytes(tokenBytes)
	// if err != nil {
	// 	return fmt.Errorf("failed to create api client config: %v", err)
	// }
	// config, err := clientConfig.ClientConfig()
	// if err != nil {
	// 	return fmt.Errorf("failed to create api client config: %v", err)
	// }

	// ca := x509.NewCertPool()
	// ca.AppendCertsFromPEM(config.CAData)
	// tlsConfig := &tls.Config{
	// 	InsecureSkipVerify: false,
	// 	RootCAs:            ca,
	// }
	// tr := &http.Transport{TLSClientConfig: tlsConfig}
	// client := http.Client{Transport: tr}

	// req, err := http.NewRequest(http.MethodGet, c.APIAddress+"/v1beta1/calico/kubeconfig", nil)
	// if err != nil {
	// 	return fmt.Errorf("can't create http request: %v", err)
	// }
	// req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.BearerToken))
	// resp, err := client.Do(req)
	// if err != nil {
	// 	return fmt.Errorf("can't download kubelet config for calico: %v", err)
	// }
	// defer resp.Body.Close()
	// if resp.StatusCode != http.StatusOK {
	// 	return fmt.Errorf("unexpected response status: %s", resp.Status)
	// }
	// b, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	return fmt.Errorf("can't read response body: %v", err)
	// }
	// if err := file.WriteContentAtomically(path, b, 0700); err != nil {
	// 	return fmt.Errorf("can't save kubeconfig for calico: %v", err)
	// }
	// posh := NewPowershell()
	// return posh.execute(fmt.Sprintf("C:\\bootstrap.ps1 -ServiceCidr \"%s\" -DNSServerIPs \"%s\"", c.CIDRRange, c.ClusterDNS))
}

func (c CalicoInstaller) Start(_ context.Context) error {
	return nil
}

func (c CalicoInstaller) Stop() error {
	return nil
}

// installCalicoPowershell is port of the original calico installer
// with droped customization and no need to download kubernetes components
// since we have staged them.
// We also skip building the pause image as we're using the semi-official MS image from mcr.microsoft.com/oss/kubernetes/pause:1.4.1
// the original script is done by Tigera, Inc
// and can be accessed over the web on https://docs.projectcalico.org/scripts/install-calico-windows.ps1
const installCalicoPowershell = `
$existingRule = Get-NetFirewallRule -DisplayName "kubectl exec 10250" -ErrorAction SilentlyContinue; if ($existingRule -eq $null) { New-NetFirewallRule -Name "KubectlExec10250" -Description "Enable kubectl exec and log" -Action Allow -LocalPort 10250 -Enabled True -DisplayName "kubectl exec 10250" -Protocol TCP -ErrorAction SilentlyContinue } else { Write-Output "The firewall rule already exists." }
`
