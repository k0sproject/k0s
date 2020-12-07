package worker

import (
	"os"
	"os/exec"
)

// PowerShell struct
type PowerShell struct {
	powerShell string
}

// New create new session
func New() *PowerShell {
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

const installCalicoPowershell = `
# Copyright (c) 2020 Tigera, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Param(
    [parameter(Mandatory = $false)] $ReleaseBaseURL="https://github.com/projectcalico/calico/releases/download/v3.17.0/",
    [parameter(Mandatory = $false)] $ReleaseFile="calico-windows-v3.17.0.zip",
    [parameter(Mandatory = $false)] $KubeVersion="",
    [parameter(Mandatory = $false)] $DownloadOnly="no",
    [parameter(Mandatory = $false)] $Datastore="kubernetes",
    [parameter(Mandatory = $false)] $EtcdEndpoints="",
    [parameter(Mandatory = $false)] $EtcdTlsSecretName="",
    [parameter(Mandatory = $false)] $EtcdKey="",
    [parameter(Mandatory = $false)] $EtcdCert="",
    [parameter(Mandatory = $false)] $EtcdCaCert="",
    [parameter(Mandatory = $false)] $ServiceCidr="10.96.0.0/12",
    [parameter(Mandatory = $false)] $DNSServerIPs="10.96.0.10",
    [parameter(Mandatory = $false)] $CalicoBackend=""
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
    PrepareDockerFile
    ipmo C:\k\hns.psm1

    # Prepare POD infra Images
    c:\k\InstallImages.ps1

    InstallK8sBinaries
}

function InstallK8sBinaries()
{
    Install-7Zip
    $Source = "" | Select Release
    $Source.Release=$KubeVersion
    InstallKubernetesBinaries -Destination $BaseDir -Source $Source
    cp c:\k\kubernetes\node\bin\*.exe c:\k
}

function GetPlatformType()
{
    # AKS
    $hnsNetwork = Get-HnsNetwork | ? Name -EQ azure
    if ($hnsNetwork.name -EQ "azure") {
        return ("aks")
    }

    # EKS
    $hnsNetwork = Get-HnsNetwork | ? Name -like "vpcbr*"
    if ($hnsNetwork.name -like "vpcbr*") {
        return ("eks")
    }

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
    param(
        [parameter(Mandatory=$true)] $CalicoNamespace,
        [parameter(Mandatory=$false)] $KubeConfigPath = "$RootDir\calico-kube-config"
    )

    if (-Not [string]::IsNullOrEmpty($CalicoBackend)) {
        return $CalicoBackend
    }

    # Auto detect backend type
    if ($Datastore -EQ "kubernetes") {
        $encap=c:\k\kubectl.exe --kubeconfig="$RootDir\calico-kube-config" get felixconfigurations.crd.projectcalico.org default -o jsonpath='{.spec.ipipEnabled}' -n $CalicoNamespace
        if ($encap -EQ "true") {
            throw "Calico on Linux has IPIP enabled. IPIP is not supported on Windows nodes."
        }

        $encap=c:\k\kubectl.exe --kubeconfig="$RootDir\calico-kube-config" get felixconfigurations.crd.projectcalico.org default -o jsonpath='{.spec.vxlanEnabled}' -n $CalicoNamespace
        if ($encap -EQ "true") {
            return ("vxlan")
        }
        return ("bgp")
    } else {
        $CalicoBackend=c:\k\kubectl.exe --kubeconfig="$RootDir\calico-kube-config" get configmap calico-config -n $CalicoNamespace -o jsonpath='{.data.calico_backend}'
        if ($CalicoBackend -EQ "vxlan") {
            return ("vxlan")
        }
        return ("bgp")
    }
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

if (-Not [string]::IsNullOrEmpty($KubeVersion) -and $platform -NE "eks") {
    PrepareKubernetes
}
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
SetConfigParameters -OldString '<your etcd endpoints>' -NewString "$EtcdEndpoints"

if (-Not [string]::IsNullOrEmpty($EtcdTlsSecretName)) {
    $calicoNs = GetCalicoNamespace
    SetupEtcdTlsFiles -SecretName "$EtcdTlsSecretName" -CalicoNamespace $calicoNs
}
SetConfigParameters -OldString '<your etcd key>' -NewString "$EtcdKey"
SetConfigParameters -OldString '<your etcd cert>' -NewString "$EtcdCert"
SetConfigParameters -OldString '<your etcd ca cert>' -NewString "$EtcdCaCert"
SetConfigParameters -OldString '<your service cidr>' -NewString $ServiceCidr
SetConfigParameters -OldString '<your dns server ips>' -NewString $DNSServerIPs

if ($platform -EQ "aks") {
    Write-Host "Setup Calico for Windows for AKS..."
    $Backend="none"
    SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="none"'
    SetConfigParameters -OldString 'KUBE_NETWORK = "Calico.*"' -NewString 'KUBE_NETWORK = "azure.*"'

    $calicoNs = GetCalicoNamespace
    GetCalicoKubeConfig -CalicoNamespace $calicoNs -SecretName 'calico-windows'
}
if ($platform -EQ "eks") {
    $awsNodeName = Invoke-RestMethod -uri http://169.254.169.254/latest/meta-data/local-hostname -ErrorAction Ignore
    Write-Host "Setup Calico for Windows for EKS, node name $awsNodeName ..."
    $Backend = "none"
    $awsNodeNameQuote = """$awsNodeName"""
    SetConfigParameters -OldString '$(hostname).ToLower()' -NewString "$awsNodeNameQuote"
    SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="none"'
    SetConfigParameters -OldString 'KUBE_NETWORK = "Calico.*"' -NewString 'KUBE_NETWORK = "vpc.*"'

    $calicoNs = GetCalicoNamespace 
    GetCalicoKubeConfig -CalicoNamespace $calicoNs -KubeConfigPath C:\ProgramData\kubernetes\kubeconfig
}
if ($platform -EQ "ec2") {
    $awsNodeName = Invoke-RestMethod -uri http://169.254.169.254/latest/meta-data/local-hostname -ErrorAction Ignore
    Write-Host "Setup Calico for Windows for AWS, node name $awsNodeName ..."
    $awsNodeNameQuote = """$awsNodeName"""
    SetConfigParameters -OldString '$(hostname).ToLower()' -NewString "$awsNodeNameQuote"

    $calicoNs = GetCalicoNamespace
    GetCalicoKubeConfig -CalicoNamespace $calicoNs
    $Backend = GetBackendType -CalicoNamespace $calicoNs

    Write-Host "Backend networking is $Backend"
    if ($Backend -EQ "bgp") {
        SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="windows-bgp"'
    }
}
if ($platform -EQ "bare-metal") {
    $calicoNs = GetCalicoNamespace
    GetCalicoKubeConfig -CalicoNamespace $calicoNs
    $Backend = GetBackendType -CalicoNamespace $calicoNs

    Write-Host "Backend networking is $Backend"
    if ($Backend -EQ "bgp") {
        SetConfigParameters -OldString 'CALICO_NETWORKING_BACKEND="vxlan"' -NewString 'CALICO_NETWORKING_BACKEND="windows-bgp"'
    }
}

if ($DownloadOnly -EQ "yes") {
    Write-Host "Dowloaded Calico for Windows. Update c:\CalicoWindows\config.ps1 and run c:\CalicoWindows\install-calico.ps1"
    Exit
}

StartCalico

if ($Backend -NE "none") {
    New-NetFirewallRule -Name KubectlExec10250 -Description "Enable kubectl exec and log" -Action Allow -LocalPort 10250 -Enabled True -DisplayName "kubectl exec 10250" -Protocol TCP -ErrorAction SilentlyContinue
}
`
