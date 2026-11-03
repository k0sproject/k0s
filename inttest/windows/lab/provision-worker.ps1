# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Prepares a fresh clone of the Windows base VM and joins it as a k0s worker.
# Mirrors the steps of the win-wsl.yaml workflow. Expects k0s.exe and
# k0s-worker-token next to it.

param(
    # MAC address of the lab NIC added by qemu-wrapper.sh, e.g. 52-54-00-77-00-02.
    [Parameter(Mandatory)] [string] $LabMac,
    # IPv4 address to assign to the lab NIC, and the length of its prefix.
    [Parameter(Mandatory)] [string] $LabIP,
    [Parameter(Mandatory)] [int] $LabPrefixLength,
    # CIDR of the lab link, and of the cluster's pods; both may reach this VM.
    [Parameter(Mandatory)] [string] $LabCIDR,
    [Parameter(Mandatory)] [string] $PodCIDR,
    # Node name. Calico and kubelet need to agree on it.
    [Parameter(Mandatory)] [string] $NodeName
)

# Commit of microsoft/SDN that hns.psm1 is taken from, and its SHA-256.
$hnsCommit = "32df9732ef6dfab07dba205440e30bdfbd8e579a"
$hnsSha256 = "a8a53ed4fac2e27c7e4268db069d4cf3129a56d466ef3bf9465fb52dcd76a29c"

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

# Windows PowerShell doesn't fail on native command errors by itself.
function Invoke-Native {
    $exe = $args[0]
    $rest = @($args | Select-Object -Skip 1) # Stays an array with one element.
    & $exe @rest
    if ($LASTEXITCODE) { throw "$exe $rest failed: $LASTEXITCODE" }
}

if ((Get-WindowsFeature Containers).InstallState -ne "Installed") {
    throw "Containers feature is not installed"
}

$lab = Get-NetAdapter | Where-Object MacAddress -EQ $LabMac
if (-not $lab) {
    throw "Lab NIC $LabMac not found, was the VM started via qemu-wrapper.sh?"
}
Write-Host "Configuring $LabIP on adapter $($lab.Name)"
Set-NetIPInterface -InterfaceIndex $lab.ifIndex -Dhcp Disabled
New-NetIPAddress -InterfaceIndex $lab.ifIndex -IPAddress $LabIP -PrefixLength $LabPrefixLength | Out-Null

# Allow inbound traffic only from the lab link and the pods, instead of
# disabling the firewall like the workflow does. Applies to all profiles.
New-NetFirewallRule -DisplayName "k0s-lab-inbound" -Direction Inbound -Action Allow -RemoteAddress $LabCIDR, $PodCIDR | Out-Null

Write-Host "Downloading HNS helpers"
Invoke-WebRequest -UseBasicParsing -OutFile hns.psm1 -Uri "https://raw.githubusercontent.com/microsoft/SDN/$hnsCommit/Kubernetes/windows/hns.psm1"
if ((Get-FileHash hns.psm1 -Algorithm SHA256).Hash -ne $hnsSha256) {
    throw "hns.psm1 does not match the expected checksum"
}
Unblock-File hns.psm1
Import-Module .\hns.psm1 -Force

# Same as the workflow, but on the lab NIC, which is where Calico's IP
# autodetection will put the VXLAN network as well.
Write-Host "Creating overlay network on adapter $($lab.Name)"
New-HNSNetwork -Type Overlay -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -Name "External" -SubnetPolicies @(@{Type = "VSID"; VSID = 9999; }) -AdapterName $lab.Name | Out-Null

$deadline = (Get-Date).AddMinutes(2)
while (-not (Get-HnsNetwork | Where-Object { $_.Name -eq "External" })) {
    if ((Get-Date) -gt $deadline) { throw "HNS network External did not show up" }
    Start-Sleep -Seconds 2
}

# Pretend that Calico for Windows already initialized its networking since the
# last boot, so that it keeps the overlay network created above instead of
# creating its own. Same trick as in the win-wsl.yaml workflow.
Write-Host "Making Calico keep the existing overlay network"
$calicoKey = "HKLM:\Software\Tigera\Calico"
New-Item -Path $calicoKey -Force | Out-Null
$bootTime = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime
Set-ItemProperty -Path $calicoKey -Name "LastBootTime" -Value $bootTime.ToString("G")

Write-Host "Installing and starting k0s worker"
Invoke-Native .\k0s.exe version
Invoke-Native .\k0s.exe install worker --token-file=k0s-worker-token --debug "--kubelet-extra-args=--hostname-override=$NodeName --node-ip=$LabIP"
Invoke-Native .\k0s.exe start
