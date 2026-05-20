$Repo = "alijamal14/council"
$Bin = "council.exe"
$InstallDir = "$env:LOCALAPPDATA\Programs\Council"

$Arch = if ([Environment]::Is64BitOperatingSystem) { "x86_64" } else {
  Write-Error "Unsupported architecture"
  exit 1
}

$Release = Invoke-RestMethod "https://api.github.com/repos/$Repo/releases/latest"
$Version = $Release.tag_name
$Archive = "council_Windows_$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"

Write-Host "Installing Council Orchestrator $Version..."

$Tmp = New-Item -ItemType Directory -Force -Path ([System.IO.Path]::GetTempPath() + [System.Guid]::NewGuid())
$Zip = Join-Path $Tmp $Archive

Invoke-WebRequest -Uri $Url -OutFile $Zip
Expand-Archive -Path $Zip -DestinationPath $Tmp -Force

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
Move-Item -Force (Join-Path $Tmp $Bin) (Join-Path $InstallDir $Bin)

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
  Write-Host "Added $InstallDir to user PATH. Please restart your terminal."
}

Write-Host "Council Orchestrator installed to $InstallDir\$Bin"
& (Join-Path $InstallDir $Bin) --version
