$PSNativeCommandUseErrorActionPreference = $true
$ErrorActionPreference = 'Stop'
$InformationPreference = 'Continue'
$ProgressPreference = 'SilentlyContinue'

function Get-EC2-Public-OpenSSH-Keys {
  [OutputType([string[]])]

  $baseUrl = 'http://169.254.169.254/latest'

  $url = "$baseUrl/api/token"
  $headers = @{ 'X-aws-ec2-metadata-token-ttl-seconds' = '30' }
  $token = Invoke-RestMethod -Method PUT -Uri $url -Headers $headers

  # Retrieve the list of public key indices; the response is "index=key-name", once per line
  $url = "$baseUrl/meta-data/public-keys"
  $headers = @{ 'X-aws-ec2-metadata-token' = $token }
  [string]$keyIndices = Invoke-RestMethod -Uri $url -Headers $headers

  [string[]]$authorizedKeys = @()

  # Iterate over the lines and get the indices
  foreach ($line in $keyIndices.Split("`n")) {
    if ($line -match '^(\d+)=.*') {
      $index = $matches[1]
      $keyUrl = "$url/$index/openssh-key"
      try {
        [string]$key = Invoke-RestMethod -Uri $keyUrl -Headers $headers
        $authorizedKeys += $key.Trim()
      }
      catch {
        [string]$msg = $_.Exception.Message
        Write-Warning "Failed to retrieve key for index ${index}: $msg"
      }
    }
  }

  return $authorizedKeys
}

[string[]]$authorizedKeys = Get-EC2-Public-OpenSSH-Keys

# Write the user's authorized_keys file
$userName = [System.Environment]::UserName
$authorizedKeysPath = New-Item ([System.IO.Path]::Combine('~', '.ssh', 'authorized_keys')) -ItemType File -Force
& icacls $authorizedKeysPath /inheritance:r | Out-Null
& icacls $authorizedKeysPath /grant "${userName}:F" /grant 'SYSTEM:R' | Out-Null
& icacls $authorizedKeysPath.Directory /inheritance:r | Out-Null
& icacls $authorizedKeysPath.Directory /grant "${userName}:F" /grant 'SYSTEM:R' | Out-Null
$authorizedKeys -join "`r`n" | Set-Content $authorizedKeysPath -Encoding ascii
Write-Information "Generated $authorizedKeysPath"
