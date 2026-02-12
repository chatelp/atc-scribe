# Download reference data files for co-atc
# Run from the project root: .\scripts\download_assets.ps1

param(
    [string]$AssetsDir = (Join-Path (Join-Path $PSScriptRoot "..") "assets")
)

$ErrorActionPreference = "Stop"

# Resolve to absolute path
$AssetsDir = (Resolve-Path $AssetsDir).Path
Write-Host "Downloading assets to: $AssetsDir" -ForegroundColor Cyan

# Helper: download a file, writing to a temp first then moving into place.
# This avoids "file in use" errors when the app has the target open.
function Get-AssetFile {
    param([string]$Uri, [string]$Dest)
    $tmp = "$Dest.tmp"
    Invoke-WebRequest -Uri $Uri -OutFile $tmp
    Copy-Item -Path $tmp -Destination $Dest -Force
    Remove-Item $tmp -ErrorAction SilentlyContinue
}

# 1. aircraft.csv from wiedehopf/tar1090-db (gzipped)
Write-Host "`n[1/6] Downloading aircraft.csv.gz..." -ForegroundColor Yellow
$aircraftGz = Join-Path $AssetsDir "aircraft.csv.gz"
$aircraftCsv = Join-Path $AssetsDir "aircraft.csv"
$aircraftTmp = "$aircraftCsv.tmp"
Invoke-WebRequest -Uri "https://github.com/wiedehopf/tar1090-db/raw/refs/heads/csv/aircraft.csv.gz" -OutFile $aircraftGz
# Decompress to temp file, then move into place
$inStream = [System.IO.File]::OpenRead($aircraftGz)
$output = [System.IO.File]::Create($aircraftTmp)
$gzip = New-Object System.IO.Compression.GZipStream($inStream, [System.IO.Compression.CompressionMode]::Decompress)
$gzip.CopyTo($output)
$gzip.Close()
$output.Close()
$inStream.Close()
Remove-Item $aircraftGz
Copy-Item -Path $aircraftTmp -Destination $aircraftCsv -Force
Remove-Item $aircraftTmp -ErrorAction SilentlyContinue
Write-Host "  -> aircraft.csv extracted" -ForegroundColor Green

# 2. airlines.dat from OpenFlights
Write-Host "`n[2/6] Downloading airlines.dat..." -ForegroundColor Yellow
Get-AssetFile "https://raw.githubusercontent.com/jpatokal/openflights/master/data/airlines.dat" (Join-Path $AssetsDir "airlines.dat")
Write-Host "  -> airlines.dat downloaded" -ForegroundColor Green

# 3. airports.csv from OurAirports
Write-Host "`n[3/6] Downloading airports.csv..." -ForegroundColor Yellow
Get-AssetFile "https://davidmegginson.github.io/ourairports-data/airports.csv" (Join-Path $AssetsDir "airports.csv")
Write-Host "  -> airports.csv downloaded" -ForegroundColor Green

# 4. airport-frequencies.csv from OurAirports
Write-Host "`n[4/6] Downloading airport-frequencies.csv..." -ForegroundColor Yellow
Get-AssetFile "https://davidmegginson.github.io/ourairports-data/airport-frequencies.csv" (Join-Path $AssetsDir "airport-frequencies.csv")
Write-Host "  -> airport-frequencies.csv downloaded" -ForegroundColor Green

# 5. runways.csv from OurAirports
Write-Host "`n[5/6] Downloading runways.csv..." -ForegroundColor Yellow
Get-AssetFile "https://davidmegginson.github.io/ourairports-data/runways.csv" (Join-Path $AssetsDir "runways.csv")
Write-Host "  -> runways.csv downloaded" -ForegroundColor Green

# 6. navaids.csv from OurAirports
Write-Host "`n[6/6] Downloading navaids.csv..." -ForegroundColor Yellow
Get-AssetFile "https://davidmegginson.github.io/ourairports-data/navaids.csv" (Join-Path $AssetsDir "navaids.csv")
Write-Host "  -> navaids.csv downloaded" -ForegroundColor Green

Write-Host "`nAll assets downloaded successfully!" -ForegroundColor Cyan
