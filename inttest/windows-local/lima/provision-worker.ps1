# Prepares a fresh clone of the Windows base VM and joins it as a k0s worker.
# Mirrors the steps of the win-wsl.yaml workflow. Expects k0s.exe and
# k0s-worker-token next to it.

$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot

if ((Get-WindowsFeature Containers).InstallState -ne "Installed") {
    throw "Containers feature is not installed"
}

# The lab NIC added by qemu-lab.sh, linking this VM to the controller VM.
$labIP = "10.77.0.2"
$lab = Get-NetAdapter | Where-Object MacAddress -EQ "52-54-00-77-00-02"
if (-not $lab) {
    throw "Lab NIC not found, was the VM started via qemu-lab.sh?"
}
Write-Host "Configuring $labIP on adapter $($lab.Name)"
Set-NetIPInterface -InterfaceIndex $lab.ifIndex -Dhcp Disabled
New-NetIPAddress -InterfaceIndex $lab.ifIndex -IPAddress $labIP -PrefixLength 24 | Out-Null

# Allow inbound traffic only from the lab link and the k0s pod CIDR, instead of
# disabling the firewall like the workflow.
New-NetFirewallRule -DisplayName "k0s-lab-inbound" -Direction Inbound -Action Allow -RemoteAddress "10.77.0.0/24", "10.244.0.0/16" | Out-Null

Write-Host "Downloading HNS helpers"
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/Microsoft/SDN/master/Kubernetes/windows/hns.psm1" -OutFile hns.psm1
Unblock-File hns.psm1
Import-Module .\hns.psm1 -Force

# Same as the workflow, but on the lab NIC, which is where Calico's IP
# autodetection will put the VXLAN network as well.
Write-Host "Creating overlay network on adapter $($lab.Name)"
New-HNSNetwork -Type Overlay -AddressPrefix "192.168.255.0/30" -Gateway "192.168.255.1" -Name "External" -SubnetPolicies @(@{Type = "VSID"; VSID = 9999; }) -AdapterName $lab.Name
Start-Sleep -Seconds 20

Write-Host "Making Calico keep the existing overlay network"
$calicoKey = "HKLM:\Software\Tigera\Calico"
New-Item -Path $calicoKey -Force | Out-Null
$bootTime = (Get-CimInstance Win32_OperatingSystem).LastBootUpTime
Set-ItemProperty -Path $calicoKey -Name "LastBootTime" -Value $bootTime.ToString("G")

# Windows PowerShell doesn't fail on native command errors by itself.
Write-Host "Installing and starting k0s worker"
& .\k0s.exe version
& .\k0s.exe install worker --token-file=k0s-worker-token --debug --kubelet-extra-args="--hostname-override=windowsworker --node-ip=$labIP"
if ($LASTEXITCODE) { throw "k0s install worker failed: $LASTEXITCODE" }
& .\k0s.exe start
if ($LASTEXITCODE) { throw "k0s start failed: $LASTEXITCODE" }
