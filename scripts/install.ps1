# install.ps1 - Installer for Council Orchestrator (Windows)
#
# Usage:
#   powershell -c "irm https://raw.githubusercontent.com/alijamal14/council/main/scripts/install.ps1 | iex"

$Repo = "alijamal14/council"
$BinaryName = "council"
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\council"

# Detect Architecture
$Arch = "x86_64"
if ($PSVersionTable.OS -like "*Windows*") {
    if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
        $Arch = "aarch64"
    }
}

# Function to get latest version
function Get-LatestVersion {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return $Release.tag_name
}

$Version = Get-LatestVersion
if (-not $Version) {
    Write-Error "Could not determine latest version."
    return
}

Write-Host "Installing Council Orchestrator $Version for Windows/$Arch..."

$AssetName = "${BinaryName}_windows_$Arch.zip"
$DownloadUrl = "https://github.com/$Repo/releases/download/$Version/$AssetName"

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -Path $InstallDir -ItemType Directory -Force | Out-Null
}

# Download and extract
$TempDir = Join-Path $env:TEMP ([Guid]::NewGuid().ToString())
New-Item -Path $TempDir -ItemType Directory | Out-Null

$ZipFile = Join-Path $TempDir "council.zip"
Write-Host "Downloading $DownloadUrl..."
Invoke-WebRequest -Uri $DownloadUrl -OutFile $ZipFile

Write-Host "Extracting to $InstallDir..."
Expand-Archive -Path $ZipFile -DestinationPath $TempDir -Force
Move-Item -Path (Join-Path $TempDir "${BinaryName}.exe") -Destination (Join-Path $InstallDir "${BinaryName}.exe") -Force

# Update User PATH
$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    Write-Host "Adding $InstallDir to User PATH..."
    [Environment]::SetEnvironmentVariable("Path", $UserPath + ";" + $InstallDir, "User")
    $env:Path += ";" + $InstallDir
}

Write-Host "Successfully installed $BinaryName to $InstallDir\$BinaryName.exe"
Write-Host "Please restart your terminal to use 'council'."
