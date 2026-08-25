param(
    [switch]$Force
)

$Host.UI.RawUI.ForegroundColor = "Green"
Write-Host "[cardinal] Uninstalling cardinal..."
$Host.UI.RawUI.ForegroundColor = "White"

$InstallDir = "$env:USERPROFILE\.cardinal\bin"
$BinPath = "$InstallDir\cardinal.exe"

if (Test-Path $BinPath) {
    Remove-Item -Force $BinPath
    Write-Host "[cardinal] Removed $BinPath" -ForegroundColor Green
} else {
    Write-Host "[cardinal] cardinal.exe not found at $BinPath" -ForegroundColor Yellow
}

$CardinalDir = "$env:USERPROFILE\.cardinal"
if (Test-Path $CardinalDir) {
    if ($Force) {
        $confirm = "y"
    } else {
        Write-Host "WARNING: This will DELETE all images, containers, and data." -ForegroundColor Red
        $confirm = Read-Host "Remove $CardinalDir? [y/N]"
    }
    if ($confirm -eq "y" -or $confirm -eq "Y") {
        Remove-Item -Recurse -Force $CardinalDir
        Write-Host "[cardinal] Removed $CardinalDir" -ForegroundColor Green
    } else {
        Write-Host "[cardinal] Skipped $CardinalDir" -ForegroundColor Yellow
    }
}

$userPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($userPath -like "*$InstallDir*") {
    $newPath = ($userPath -split ";" | Where-Object { $_ -ne $InstallDir }) -join ";"
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    Write-Host "[cardinal] Removed $InstallDir from PATH" -ForegroundColor Green
}

Write-Host "[cardinal] cardinal uninstalled." -ForegroundColor Green
