$ErrorActionPreference = 'Stop'
$InformationPreference = 'Continue'
$ProgressPreference = 'SilentlyContinue'

# Install OpenSSH
try {
  $openSsh = (Get-WindowsCapability -Online | Where-Object Name -like 'OpenSSH.Server*')[0]
  $name = $openSsh.Name
  if ($openSsh.State -ne 'Installed') {
    Write-Information "Installing OpenSSH Server: $name"
    $installed = $openSsh | Add-WindowsCapability -Online
    $restartNeeded = $installed.RestartNeeded
    Write-Information "Installed OpenSSH Server (RestartNeeded: $restartNeeded)"
  }
  else {
    Write-Information "OpenSSH Server is installed: $name"
  }
}
catch {
  [string]$msg = $_.Exception.Message
  Write-Warning "Failed to install OpenSSH Server: $msg"
}

# Write OpenSSH sshd config file
[string[]]$sshdConfig = @(
  'PubkeyAuthentication    yes'
  'AuthorizedKeysFile      .ssh/authorized_keys'
  'PasswordAuthentication  no'
  'PermitEmptyPasswords    no'
  'Subsystem       sftp    sftp-server.exe'
)
if (!(Test-Path -PathType Container $env:ProgramData)) {
  throw "%ProgramData% is not a directory: $env:ProgramData"
}
for ($attempt = 1; $true; $attempt++) {
  $sshConfigDir = [System.IO.Path]::Combine($env:ProgramData, 'ssh')
  if (Test-Path -PathType Container $sshConfigDir) {
    $sshdConfig -join "`r`n" | Set-Content -Path $env:ProgramData\ssh\sshd_config -Encoding ascii
    break
  }
  if ($attempt -gt 30) {
    throw "OpenSSH config directory doesn't exist: $sshConfigDir"
  }
  Start-Sleep -Seconds 1
}

# Start OpenSSH Server
Set-Service -Name sshd -StartupType Automatic
if ((Get-Service sshd).Status -ne 'Running') {
  Write-Information 'Starting sshd service ...'
  Start-Service sshd
}
else {
  Write-Information 'Service sshd is already running'
}

Write-Information 'Done'
