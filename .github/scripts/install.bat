@echo off
setlocal enabledelayedexpansion

:: Amo CLI Installation Script for Windows
:: This batch file downloads and executes the PowerShell installation script

echo ==========================================
echo       Amo CLI Installation Script
echo ==========================================
echo.

:: Check if PowerShell is available
powershell -Command "Get-Host" >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: PowerShell is not available or not working properly.
    echo Please install PowerShell or use the manual installation method.
    echo.
    echo Manual installation:
    echo 1. Download: https://cli.release.amo.run/amo_windows_amd64.exe
    echo 2. Rename to amo.exe
    echo 3. Add to PATH
    pause
    exit /b 1
)

echo Detecting system architecture...
if "%PROCESSOR_ARCHITECTURE%"=="AMD64" (
    set "ARCH=amd64"
) else if "%PROCESSOR_ARCHITEW6432%"=="AMD64" (
    set "ARCH=amd64"
) else if "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
    echo WARNING: ARM64 detected but not fully supported yet. Using AMD64 binary.
    set "ARCH=amd64"
) else (
    echo ERROR: Unsupported architecture: %PROCESSOR_ARCHITECTURE%
    echo Please download manually from: https://cli.release.amo.run/
    pause
    exit /b 1
)

echo Architecture: %ARCH%
echo.

:: Check for internet connectivity
echo Checking internet connectivity...
ping -n 1 github.com >nul 2>&1
if %errorlevel% neq 0 (
    echo ERROR: No internet connection or cannot reach GitHub.
    echo Please check your internet connection and try again.
    pause
    exit /b 1
)

echo Internet connection: OK
echo.

:: Ask user for installation preference
echo Choose installation method:
echo 1. Run PowerShell installation script (Recommended)
echo 2. Download binary manually
echo.
set /p choice="Enter your choice (1 or 2): "

if "%choice%"=="1" (
    echo.
    echo Running PowerShell installation script...
    echo This will download and install Amo CLI automatically.
    echo.
    
    :: Execute PowerShell installation script
    powershell -ExecutionPolicy Bypass -Command "iex ((New-Object System.Net.WebClient).DownloadString('https://cli.release.amo.run/install.ps1'))"
    
    if !errorlevel! equ 0 (
        echo.
        echo Installation completed successfully!
        echo You can now use 'amo' command from PowerShell or Command Prompt.
    ) else (
        echo.
        echo Installation failed. Please try manual installation.
        echo Download from: https://cli.release.amo.run/amo_windows_amd64.exe
    )
    
) else if "%choice%"=="2" (
    echo.
    echo Manual installation instructions:
    echo 1. Download: https://cli.release.amo.run/amo_windows_amd64.exe
    echo 2. Save it as 'amo.exe' in a directory of your choice
    echo 3. Add that directory to your PATH environment variable
    echo.
    echo Opening download URL in your default browser...
    start https://cli.release.amo.run/amo_windows_amd64.exe
    
) else (
    echo.
    echo Invalid choice. Please run the script again and choose 1 or 2.
)

echo.
echo Quick start after installation:
echo   amo --help         # Show help
echo   amo workflow list  # List available workflows  
echo   amo tool list      # List available tools
echo.

pause 