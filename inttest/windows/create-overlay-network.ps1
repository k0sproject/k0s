# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Pre-creates the HNS overlay network that Calico for Windows uses, on the given
# network adapter, and makes Calico keep it instead of creating its own. Used
# by the win-wsl.yaml workflow and by the local Windows lab (lab/lab.sh).

param(
    # Name of the network adapter that carries the VXLAN traffic.
    [Parameter(Mandatory)] [string] $AdapterName
)

# Commit of microsoft/SDN that hns.psm1 is taken from, and its SHA-256.
$hnsCommit = "32df9732ef6dfab07dba205440e30bdfbd8e579a"
$hnsSha256 = "a8a53ed4fac2e27c7e4268db069d4cf3129a56d466ef3bf9465fb52dcd76a29c"

$ErrorActionPreference = "Stop"

Write-Host "Downloading HNS helpers"
$hnsModule = Join-Path $env:TEMP "hns.psm1"
Invoke-WebRequest -UseBasicParsing -OutFile $hnsModule -Uri "https://raw.githubusercontent.com/microsoft/SDN/$hnsCommit/Kubernetes/windows/hns.psm1"
if ((Get-FileHash $hnsModule -Algorithm SHA256).Hash -ne $hnsSha256) {
    throw "hns.psm1 does not match the expected checksum"
}
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
Unblock-File $hnsModule
Import-Module $hnsModule -Force

Write-Host "Creating overlay network on adapter $AdapterName"
New-HNSNetwork -Type Overlay -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -Name "External" -SubnetPolicies @(@{Type = "VSID"; VSID = 9999; }) -AdapterName $AdapterName | Out-Null

$deadline = (Get-Date).AddMinutes(2)
while (-not (Get-HnsNetwork | Where-Object { $_.Name -eq "External" })) {
    if ((Get-Date) -gt $deadline) { throw "HNS network External did not show up" }
    Start-Sleep -Seconds 2
}
# Creating the network puts the adapter behind a new virtual switch, which
# briefly interrupts its connectivity. Give it time to settle.
Start-Sleep -Seconds 20

# Pretend that Calico for Windows already initialized its networking since the
# last boot, so that it keeps the overlay network created above instead of
# creating its own. Calico reads the value back as a string in the "general"
# date format.
Write-Host "Making Calico keep the existing overlay network"
$calicoKey = "HKLM:\Software\Tigera\Calico"
New-Item -Path $calicoKey -Force | Out-Null
$bootTime = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime
if (-not $bootTime) { throw "Failed to get last boot time" }
Set-ItemProperty -Path $calicoKey -Name "LastBootTime" -Value $bootTime.ToString("G")
