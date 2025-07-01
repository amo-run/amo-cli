#Requires -Version 5.1

<#
.SYNOPSIS
    Amo CLI Installation Script for Windows
.DESCRIPTION
    Downloads and installs the latest Amo CLI binary for Windows.
    Supports AMD64 architecture.
.PARAMETER InstallDir
    Directory to install the binary (default: $env:LOCALAPPDATA\amo\bin)
.PARAMETER AddToPath
    Add the install directory to the user's PATH environment variable
.PARAMETER Force
    Overwrite existing installation without prompting
.EXAMPLE
    .\install.ps1
.EXAMPLE
    .\install.ps1 -InstallDir "C:\Tools\amo" -AddToPath
.EXAMPLE
    iex ((New-Object System.Net.WebClient).DownloadString('https://cli.release.amo.run/install.ps1'))
#>

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\amo\bin",
    [switch]$AddToPath,
    [switch]$Force
)

# Configuration
$ErrorActionPreference = "Stop"
$ProgressPreference = "SilentlyContinue"

$Config = @{
    GitHubRepo = "amo-run/amo-cli"
    BinaryName = "amo.exe"
    BaseURL = "https://cli.release.amo.run"
    Platform = "windows"
    Architecture = "amd64"
}

# Colors for output
$Colors = @{
    Info = "Cyan"
    Success = "Green"
    Warning = "Yellow"
    Error = "Red"
}

# Helper functions
function Write-Log {
    param(
        [string]$Message,
        [string]$Level = "Info"
    )
    
    $timestamp = Get-Date -Format "HH:mm:ss"
    $color = $Colors[$Level]
    
    switch ($Level) {
        "Info" { Write-Host "[$timestamp] [INFO] $Message" -ForegroundColor $color }
        "Success" { Write-Host "[$timestamp] [SUCCESS] $Message" -ForegroundColor $color }
        "Warning" { Write-Host "[$timestamp] [WARNING] $Message" -ForegroundColor $color }
        "Error" { Write-Host "[$timestamp] [ERROR] $Message" -ForegroundColor $color }
    }
}

function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    $arch6432 = $env:PROCESSOR_ARCHITEW6432
    
    if ($arch6432 -eq "AMD64" -or $arch -eq "AMD64") {
        return "amd64"
    } elseif ($arch -eq "ARM64") {
        Write-Log "ARM64 architecture detected, but not supported yet. Using AMD64 binary." -Level "Warning"
        return "amd64"
    } else {
        throw "Unsupported architecture: $arch"
    }
}

function Test-InternetConnection {
    try {
        $null = Invoke-WebRequest -Uri "https://github.com" -Method Head -TimeoutSec 10
        return $true
    } catch {
        return $false
    }
}

function Get-FileChecksum {
    param([string]$FilePath)
    
    $hasher = [System.Security.Cryptography.SHA256]::Create()
    try {
        $stream = [System.IO.File]::OpenRead($FilePath)
        $hashBytes = $hasher.ComputeHash($stream)
        return [System.BitConverter]::ToString($hashBytes).Replace('-', '').ToLower()
    } finally {
        if ($stream) { $stream.Dispose() }
        $hasher.Dispose()
    }
}

function Confirm-Checksum {
    param(
        [string]$FilePath,
        [string]$ChecksumUrl,
        [string]$BinaryFile
    )
    
    try {
        Write-Log "Verifying checksum..."
        
        # Try primary checksum URL first, then mirror
        try {
            $checksumContent = Invoke-WebRequest -Uri $ChecksumUrl -UseBasicParsing | Select-Object -ExpandProperty Content
            $expectedChecksum = ($checksumContent -split '\s+')[0].ToLower()
            $actualChecksum = Get-FileChecksum -FilePath $FilePath
            
            if ($expectedChecksum -eq $actualChecksum) {
                Write-Log "Checksum verification passed" -Level "Success"
                return $true
            } else {
                Write-Log "Checksum verification failed, but continuing installation" -Level "Warning"
                return $false
            }
        } catch {
            # Try mirror checksum
            Write-Log "Primary checksum failed, trying mirror..." -Level "Warning"
            $mirrorChecksumUrl = "https://toolchains.mirror.toulan.fun/amo-run/amo-cli/latest/$BinaryFile.sha256"
            
            try {
                $checksumContent = Invoke-WebRequest -Uri $mirrorChecksumUrl -UseBasicParsing | Select-Object -ExpandProperty Content
                $expectedChecksum = ($checksumContent -split '\s+')[0].ToLower()
                $actualChecksum = Get-FileChecksum -FilePath $FilePath
                
                if ($expectedChecksum -eq $actualChecksum) {
                    Write-Log "Checksum verification passed (from mirror)" -Level "Success"
                    return $true
                } else {
                    Write-Log "Mirror checksum verification failed, but continuing installation" -Level "Warning"
                    return $false
                }
            } catch {
                Write-Log "Could not verify checksum from both primary and mirror sources: $($_.Exception.Message)" -Level "Warning"
                return $false
            }
        }
    } catch {
        Write-Log "Could not verify checksum: $($_.Exception.Message)" -Level "Warning"
        return $false
    }
}

function Add-ToPath {
    param([string]$Directory)
    
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    $paths = $userPath -split ';' | Where-Object { $_ -ne "" }
    
    if ($Directory -notin $paths) {
        $newPath = ($paths + $Directory) -join ';'
        [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
        Write-Log "Added $Directory to user PATH" -Level "Success"
        Write-Log "Please restart your terminal or reload your profile for PATH changes to take effect" -Level "Info"
        return $true
    } else {
        Write-Log "$Directory is already in PATH" -Level "Info"
        return $false
    }
}

function Install-AmoCLI {
    Write-Log "Amo CLI Installation Script for Windows"
    Write-Log "======================================="
    
    # Check internet connection
    if (-not (Test-InternetConnection)) {
        throw "No internet connection available"
    }
    
    # Detect architecture
    $arch = Get-Architecture
    Write-Log "Detected architecture: $arch"
    
    # Set binary filename and URLs
    $binaryFile = "amo_$($Config.Platform)_$arch.exe"
    $downloadUrl = "$($Config.BaseURL)/$binaryFile"
    $checksumUrl = "$($Config.BaseURL)/$binaryFile.sha256"
    
    Write-Log "Download URL: $downloadUrl"
    
    # Create install directory
    if (-not (Test-Path $InstallDir)) {
        Write-Log "Creating install directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    $installPath = Join-Path $InstallDir $Config.BinaryName
    
    # Check if already installed
    if (Test-Path $installPath) {
        if (-not $Force) {
            $response = Read-Host "Amo CLI is already installed. Overwrite? (y/N)"
            if ($response -notmatch "^[Yy]") {
                Write-Log "Installation cancelled by user"
                return
            }
        }
        Write-Log "Removing existing installation..."
        Remove-Item $installPath -Force
    }
    
    # Download binary
    Write-Log "Downloading $($Config.BinaryName)..."
    $tempFile = [System.IO.Path]::GetTempFileName()
    
    try {
        # Try original URL first, then mirror if it fails
        try {
            Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile -UseBasicParsing
            Write-Log "Downloaded from primary source" -Level "Success"
        } catch {
            Write-Log "Primary download failed, trying mirror site..." -Level "Warning"
            $mirrorUrl = "https://toolchains.mirror.toulan.fun/amo-run/amo-cli/latest/$binaryFile"
            Write-Log "Mirror URL: $mirrorUrl"
            
            try {
                Invoke-WebRequest -Uri $mirrorUrl -OutFile $tempFile -UseBasicParsing
                Write-Log "Downloaded from mirror site ✓" -Level "Success"
            } catch {
                throw "Both primary and mirror downloads failed: Primary=$($_.Exception.Message)"
            }
        }
        
        # Check if file was downloaded successfully
        if (-not (Test-Path $tempFile) -or (Get-Item $tempFile).Length -eq 0) {
            throw "Downloaded file is empty or doesn't exist"
        }
        
        # Verify checksum
        Confirm-Checksum -FilePath $tempFile -ChecksumUrl $checksumUrl -BinaryFile $binaryFile
        
        # Install binary
        Write-Log "Installing $($Config.BinaryName) to $installPath"
        Move-Item $tempFile $installPath -Force
        
        # Verify installation
        if (Test-Path $installPath) {
            Write-Log "Installation completed successfully!" -Level "Success"
            
            # Add to PATH if requested
            if ($AddToPath) {
                Add-ToPath -Directory $InstallDir
            }
            
            # Try to get version
            try {
                $version = & $installPath --version 2>$null
                Write-Log "Installed version: $version"
            } catch {
                Write-Log "Binary installed but could not get version"
            }
            
            Write-Log ""
            Write-Log "Quick start:" -Level "Success"
            Write-Log "  $installPath --help         # Show help"
            Write-Log "  $installPath workflow list  # List available workflows"
            Write-Log "  $installPath tool list      # List available tools"
            
            if (-not $AddToPath) {
                Write-Log ""
                Write-Log "To add Amo CLI to your PATH, run:" -Level "Info"
                Write-Log "  `$env:PATH += ';$InstallDir'"
                Write-Log "Or rerun this script with -AddToPath parameter"
            }
            
        } else {
            throw "Installation failed: binary not found at $installPath"
        }
        
    } catch {
        Write-Log "Installation failed: $($_.Exception.Message)" -Level "Error"
        throw
    } finally {
        # Cleanup temp file
        if (Test-Path $tempFile) {
            Remove-Item $tempFile -Force -ErrorAction SilentlyContinue
        }
    }
    
    Write-Log "Installation completed! 🎉" -Level "Success"
}

# Main execution
try {
    Install-AmoCLI
} catch {
    Write-Log $_.Exception.Message -Level "Error"
    exit 1
} 