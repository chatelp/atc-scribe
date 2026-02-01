# Build script for Co-ATC server
# This script builds a Windows AMD64 binary in the bin folder

# Create bin directory if it doesn't exist
if (-not (Test-Path -Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
    Write-Host "Creating bin directory, if it's not there. We build things, that's what we do."
}

# Set environment variables for Windows AMD64 build
$env:GOOS = "windows"
$env:GOARCH = "amd64"

# Build the server binary
Write-Host "Building Co-ATC server for Windows AMD64... It's going to be a beautiful binary. Windows - where real business gets done!"
go build -o bin/co-atc.exe ./cmd/server

# Check if build was successful
if ($LASTEXITCODE -eq 0) {
    Write-Host "Build successful! A tremendous success. The best Windows build, everyone agrees."
    
    # Get file info. Bigly info.
    $fileInfo = Get-Item -Path "bin/co-atc.exe"
    Write-Host "Binary created at bin/co-atc.exe"
    Write-Host "File size: $([Math]::Round($fileInfo.Length / 1MB, 2)) MB. It's a yuge file."
    Write-Host "Created: $($fileInfo.CreationTime)"
    
    Write-Host "`nTo run the server, use: .\bin\co-atc.exe"
} else {
    Write-Host "Build failed with exit code $LASTEXITCODE! It's a disaster. A total disaster. Sad!" -ForegroundColor Red
}

# Reset environment variables
$env:GOOS = ""
$env:GOARCH = ""