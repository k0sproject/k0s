# Installs the Containers feature. Exits with 3010 (ERROR_SUCCESS_REBOOT_REQUIRED)
# after scheduling the reboot that's needed to finish the installation, and with
# 0 once the feature is installed.

$ErrorActionPreference = "Stop"

$state = (Get-WindowsFeature Containers).InstallState
Write-Host "Containers feature: $state"
if ($state -eq "Installed") {
    exit 0
}

if ($state -ne "InstallPending") {
    Install-WindowsFeature Containers | Out-Null
}
# Delay the reboot so that this script can return over SSH first.
shutdown.exe /r /t 10 /d p:4:1 /c "k0s lab: Containers feature"
exit 3010
