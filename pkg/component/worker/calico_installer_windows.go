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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/k0sproject/k0s/internal/pkg/file"
	"github.com/k0sproject/k0s/pkg/token"

	"k8s.io/client-go/tools/clientcmd"

	"github.com/Microsoft/hcsshim"
	"github.com/avast/retry-go"
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
	path := "C:\\bootstrap.ps1"

	if err := os.Mkdir("C:\\CalicoWindows", 777); err != nil {
		if os.IsExist(err) {
			logrus.Warn("CalicoWindows already set up")
			return nil
		}
		return fmt.Errorf("can't create CalicoWindows dir: %v", err)
	}

	if err := file.WriteContentAtomically(path, []byte(installCalicoPowershell), 777); err != nil {
		return fmt.Errorf("can't unpack calico installer: %v", err)
	}

	if err := c.SaveKubeConfig("C:\\calico-kube-config"); err != nil {
		return fmt.Errorf("can't get calico-kube-config: %v", err)
	}

	return nil
}

func (c CalicoInstaller) SaveKubeConfig(path string) error {
	tokenBytes, err := token.DecodeJoinToken(c.Token)
	if err != nil {
		return fmt.Errorf("failed to decode token: %v", err)
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(tokenBytes)
	if err != nil {
		return fmt.Errorf("failed to create api client config: %v", err)
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("failed to create api client config: %v", err)
	}

	ca := x509.NewCertPool()
	ca.AppendCertsFromPEM(config.CAData)
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		RootCAs:            ca,
	}
	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := http.Client{Transport: tr}

	req, err := http.NewRequest(http.MethodGet, c.APIAddress+"/v1beta1/calico/kubeconfig", nil)
	if err != nil {
		return fmt.Errorf("can't create http request: %v", err)
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", config.BearerToken))
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("can't download kubelet config for calico: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected response status: %s", resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("can't read response body: %v", err)
	}
	if err := file.WriteContentAtomically(path, b, 0700); err != nil {
		return fmt.Errorf("can't save kubeconfig for calico: %v", err)
	}
	posh := NewPowershell()
	return posh.execute(fmt.Sprintf("C:\\bootstrap.ps1 -ServiceCidr \"%s\" -DNSServerIPs \"%s\"", c.CIDRRange, c.ClusterDNS))
}

func (c CalicoInstaller) Start(_ context.Context) error {
	return nil
}

func (c CalicoInstaller) Stop() error {
	return nil
}

// PowerShell struct
type PowerShell struct {
	powerShell string
}

// New create new session
func NewPowershell() *PowerShell {
	ps, _ := exec.LookPath("powershell.exe")
	return &PowerShell{
		powerShell: ps,
	}
}

func (p *PowerShell) execute(args ...string) error {
	args = append([]string{"-NoProfile", "-NonInteractive"}, args...)
	cmd := exec.Command(p.powerShell, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func getSourceVip() (string, error) {
	var vip string

	err := retry.Do(func() error {
		ep, err := hcsshim.GetHNSEndpointByName("Calico_ep")
		if err != nil {
			logrus.WithError(err).Warn("can't get Calico_ep endpoint")
			return err
		}
		vip = ep.IPAddress.String()
		return nil
	}, retry.Delay(time.Second*5))
	if err != nil {
		return "", err
	}
	return vip, nil
}

// installCalicoPowershell is port of the original calico installer
// with droped customization and no need to download kubernetes components
// since we have staged them.
// We also skip building the pause image as we're using the semi-official MS image from mcr.microsoft.com/oss/kubernetes/pause:1.4.1
// the original script is done by Tigera, Inc
// and can be accessed over the web on https://docs.projectcalico.org/scripts/install-calico-windows.ps1
const installCalicoPowershell = `
# Copyright 2020 k0s authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Param(
    [parameter(Mandatory = $false)] $ReleaseBaseURL="https://github.com/projectcalico/calico/releases/download/v3.17.0/",
    [parameter(Mandatory = $false)] $ReleaseFile="calico-windows-v3.17.0.zip",
    [parameter(Mandatory = $false)] $Datastore="kubernetes",
    [parameter(Mandatory = $false)] $ServiceCidr="10.96.0.0/12",
    [parameter(Mandatory = $false)] $DNSServerIPs="10.96.0.10"
)

function DownloadFiles()
{
    Write-Host "Downloading CNI binaries"
    md $BaseDir\cni\config -ErrorAction Ignore
    DownloadFile -Url  "https://github.com/Microsoft/SDN/raw/master/Kubernetes/flannel/l2bridge/cni/host-local.exe" -Destination $BaseDir\cni\host-local.exe

    Write-Host "Downloading Windows Kubernetes scripts"
    DownloadFile -Url  https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/hns.psm1 -Destination $BaseDir\hns.psm1
    DownloadFile -Url  https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/InstallImages.ps1 -Destination $BaseDir\InstallImages.ps1
    DownloadFile -Url  https://github.com/Microsoft/SDN/raw/master/Kubernetes/windows/Dockerfile -Destination $BaseDir\Dockerfile
}

function PrepareDockerFile()
{
    # Update Dockerfile for windows
    $OSInfo = (Get-ComputerInfo  | select WindowsVersion, OsBuildNumber)
    $OSNumber = $OSInfo.WindowsVersion
    $ExistOSNumber = cat c:\k\Dockerfile | findstr.exe $OSNumber
    if (!$ExistOSNumber)
    {
        Write-Host "Update dockerfile for $OSNumber"

        $ImageWithOSNumber = "nanoserver:" + $OSNumber
        (get-content c:\k\Dockerfile) | foreach-object {$_ -replace "nanoserver", "$ImageWithOSNumber"} | set-content c:\k\Dockerfile
    }
}

function PrepareKubernetes()
{
    DownloadFiles
    ipmo C:\k\hns.psm1

}

function GetPlatformType()
{
 	# EC2
    $restError = $null
    Try {
        $awsNodeName=Invoke-RestMethod -uri http://169.254.169.254/latest/meta-data/local-hostname -ErrorAction Ignore
    } Catch {
        $restError = $_
    }
    if ($restError -eq $null) {
        return ("ec2")
    }

    return ("bare-metal")
}

function GetBackendType()
{
	return ("vxlan")
}

function GetCalicoNamespace() {

    return ("kube-system")
}

function GetCalicoKubeConfig()
{
    param(
      [parameter(Mandatory=$true)] $CalicoNamespace,
      [parameter(Mandatory=$false)] $SecretName = "calico-node",
      [parameter(Mandatory=$false)] $KubeConfigPath = "C:\\var\\lib\\k0s\\kubelet-bootstrap.conf"
    )
	Move-Item -Path C:\\calico-kube-config -Destination C:\\CalicoWindows\calico-kube-config
}


function SetConfigParameters {
    param(
        [parameter(Mandatory=$true)] $OldString,
        [parameter(Mandatory=$true)] $NewString
    )

    (Get-Content $RootDir\config.ps1).replace($OldString, $NewString) | Set-Content $RootDir\config.ps1 -Force
}

function StartCalico()
{
    Write-Host "Start Calico for Windows..."

    pushd
    cd $RootDir
    .\install-calico.ps1
    popd
    Write-Host "Calico for Windows Started"
}

$BaseDir="c:\k"
$RootDir="c:\CalicoWindows"
$CalicoZip="c:\calico-windows.zip"

[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$helper = "$BaseDir\helper.psm1"
$helperv2 = "$BaseDir\helper.v2.psm1"
md $BaseDir -ErrorAction Ignore
if (!(Test-Path $helper))
{
    Invoke-WebRequest https://raw.githubusercontent.com/Microsoft/SDN/master/Kubernetes/windows/helper.psm1 -O $BaseDir\helper.psm1
}
if (!(Test-Path $helperv2))
{
    Invoke-WebRequest https://raw.githubusercontent.com/Microsoft/SDN/master/Kubernetes/windows/helper.v2.psm1 -O $BaseDir\helper.v2.psm1
}
ipmo -force $helper
ipmo -force $helperv2

$platform=GetPlatformType

PrepareKubernetes

Write-Host "Download Calico for Windows release..."
DownloadFile -Url $ReleaseBaseURL/$ReleaseFile -Destination c:\calico-windows.zip

if ((Get-Service | where Name -Like 'Calico*' | where Status -EQ Running) -NE $null) {
Write-Host "Calico services are still running. In order to re-run the installation script, stop the CalicoNode and CalicoFelix services or uninstall them by running: $RootDir\uninstall-calico.ps1"
Exit
}

Remove-Item $RootDir -Force  -Recurse -ErrorAction SilentlyContinue
Write-Host "Unzip Calico for Windows release..."
Expand-Archive $CalicoZip c:\

Write-Host "Setup Calico for Windows..."
SetConfigParameters -OldString '<your datastore type>' -NewString $Datastore


SetConfigParameters -OldString '<your service cidr>' -NewString $ServiceCidr
SetConfigParameters -OldString '<your dns server ips>' -NewString $DNSServerIPs


if ($platform -EQ "ec2") {
    $awsNodeName = Invoke-RestMethod -uri http://169.254.169.254/latest/meta-data/local-hostname -ErrorAction Ignore
    Write-Host "Setup Calico for Windows for AWS, node name $awsNodeName ..."
    $awsNodeNameQuote = """$awsNodeName"""
    SetConfigParameters -OldString '$(hostname).ToLower()' -NewString "$awsNodeNameQuote"

    $calicoNs = GetCalicoNamespace
    GetCalicoKubeConfig -CalicoNamespace $calicoNs
    $Backend = GetBackendType

    Write-Host "Backend networking is $Backend"
    if ($Backend -EQ "bgp") {
        SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="windows-bgp"'
    }
}
if ($platform -EQ "bare-metal") {
    $calicoNs = GetCalicoNamespace
    GetCalicoKubeConfig -CalicoNamespace $calicoNs
    $Backend = GetBackendType

    Write-Host "Backend networking is $Backend"
    if ($Backend -EQ "bgp") {
        SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="windows-bgp"'
    }
}

StartCalico

New-NetFirewallRule -Name KubectlExec10250 -Description "Enable kubectl exec and log" -Action Allow -LocalPort 10250 -Enabled True -DisplayName "kubectl exec 10250" -Protocol TCP -ErrorAction SilentlyContinue
`
