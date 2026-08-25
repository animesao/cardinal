param(
    [string]$InstallDir = "$env:USERPROFILE\.cardinal\bin",
    [string]$GoVersion = "1.22.5",
    [switch]$NoPath
)

# Exit early on any error so partial installs leave a clean state.
$ErrorActionPreference = "Stop"

$Host.UI.RawUI.ForegroundColor = "Green"
Write-Host "[cardinal] cardinal - Simple Container Runtime Installer"
$Host.UI.RawUI.ForegroundColor = "White"
Write-Host ""

$CardinalDir = "$env:USERPROFILE\.cardinal"
$BinPath = "$InstallDir\cardinal.exe"

$Host.UI.RawUI.ForegroundColor = "Green"
Write-Host "[cardinal] Installing to: $BinPath"
$Host.UI.RawUI.ForegroundColor = "White"

function Refresh-Path {
    $env:Path = [Environment]::GetEnvironmentVariable("Path", "Machine") + ";" + [Environment]::GetEnvironmentVariable("Path", "User")
}

function Install-Go-MSI {
    param([string]$Version)
    $goUrl = "https://go.dev/dl/go$Version.windows-amd64.msi"
    $goInstaller = "$env:TEMP\go-install-$Version.msi"
    try {
        [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
        Write-Host "[cardinal] Downloading Go $Version MSI..."
        Invoke-WebRequest -Uri $goUrl -OutFile $goInstaller -UseBasicParsing
        Write-Host "[cardinal] Installing Go $Version (requires admin)..."
        $proc = Start-Process msiexec -ArgumentList "/i `"$goInstaller`" /quiet /norestart" -Wait -PassThru -NoNewWindow
        if ($proc.ExitCode -ne 0 -and $proc.ExitCode -ne 3010) {
            throw "MSI installer exited with code $($proc.ExitCode)"
        }
        Refresh-Path
        Remove-Item -Force $goInstaller -ErrorAction SilentlyContinue
    } catch {
        Write-Host "[cardinal] Go MSI installation failed: $_" -ForegroundColor Red
        Write-Host "[cardinal] Install Go manually from https://go.dev/dl/" -ForegroundColor Yellow
        exit 1
    }
}

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    $Host.UI.RawUI.ForegroundColor = "Yellow"
    Write-Host "[cardinal] Go not found. Installing Go $GoVersion..."
    $Host.UI.RawUI.ForegroundColor = "White"

    if (Get-Command winget -ErrorAction SilentlyContinue) {
        Write-Host "[cardinal] Installing Go via winget..."
        winget install GoLang.Go --silent --accept-package-agreements 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Host "[cardinal] winget failed (exit $LASTEXITCODE), falling back to MSI..." -ForegroundColor Yellow
            Install-Go-MSI -Version $GoVersion
        } else {
            Refresh-Path
        }
    } else {
        Install-Go-MSI -Version $GoVersion
    }

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        Write-Host "[cardinal] Go was installed but not found in PATH." -ForegroundColor Yellow
        Write-Host "[cardinal] Restart your terminal or refresh PATH manually." -ForegroundColor Yellow
        exit 1
    }
    Write-Host "[cardinal] Go installed: $(go version)" -ForegroundColor Green
}

# ---- Clone repo ----
$TmpDir = "$env:TEMP\cardinal-build"
if (Test-Path $TmpDir) { Remove-Item -Recurse -Force $TmpDir }
Write-Host "[cardinal] Cloning cardinal repository..."
git clone --depth 1 "https://github.com/animesao/cardinal.git" $TmpDir 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Host "[cardinal] Git clone failed!" -ForegroundColor Red
    exit 1
}

Set-Location $TmpDir

Write-Host "[cardinal] Building cardinal..."
$env:CGO_ENABLED = "0"
go build -ldflags="-s -w" -o cardinal.exe .
if ($LASTEXITCODE -ne 0) {
    Write-Host "[cardinal] Build failed!" -ForegroundColor Red
    exit 1
}

if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
}

Move-Item -Force cardinal.exe "$BinPath"
Write-Host "[cardinal] Binary installed to $BinPath" -ForegroundColor Green

Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue

if (-not $NoPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("Path", "$userPath;$InstallDir", "User")
        Write-Host "[cardinal] Added to PATH (user)" -ForegroundColor Yellow
        Write-Host "[cardinal] Restart terminal or run: `$env:Path += ';$InstallDir'" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "[cardinal] Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "[cardinal] Quick start:"
Write-Host "[cardinal]   cardinal pull alpine"
Write-Host "[cardinal]   cardinal run --rm alpine echo hello"
Write-Host "[cardinal]   cardinal --help"
Write-Host ""
