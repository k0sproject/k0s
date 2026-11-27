# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Prepares a fresh clone of the Windows base VM and joins it as a k0s worker.
# Mirrors the steps of the win-wsl.yaml workflow. Expects k0s.exe,
# k0s-worker-token and create-overlay-network.ps1 next to it.

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

# Same as the workflow, but on the lab NIC, which is where Calico's IP
# autodetection will put the VXLAN network as well.
& .\create-overlay-network.ps1 -AdapterName $lab.Name

Write-Host "Installing and starting k0s worker"
Invoke-Native .\k0s.exe version
Invoke-Native .\k0s.exe install worker --token-file=k0s-worker-token --debug "--kubelet-extra-args=--hostname-override=$NodeName --node-ip=$LabIP"
Invoke-Native .\k0s.exe start
