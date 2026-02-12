#!/usr/bin/env bash
# Download reference data files for co-atc
# Run from the project root: ./scripts/download_assets.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="${1:-$SCRIPT_DIR/../assets}"
ASSETS_DIR="$(cd "$ASSETS_DIR" 2>/dev/null && pwd || mkdir -p "$ASSETS_DIR" && cd "$ASSETS_DIR" && pwd)"

echo "Downloading assets to: $ASSETS_DIR"

# 1. aircraft.csv from wiedehopf/tar1090-db (gzipped)
echo -e "\n[1/6] Downloading aircraft.csv.gz..."
curl -fSL "https://github.com/wiedehopf/tar1090-db/raw/refs/heads/csv/aircraft.csv.gz" -o "$ASSETS_DIR/aircraft.csv.gz"
gunzip -f "$ASSETS_DIR/aircraft.csv.gz"
echo "  -> aircraft.csv extracted"

# 2. airlines.dat from OpenFlights
echo -e "\n[2/6] Downloading airlines.dat..."
curl -fSL "https://raw.githubusercontent.com/jpatokal/openflights/master/data/airlines.dat" -o "$ASSETS_DIR/airlines.dat"
echo "  -> airlines.dat downloaded"

# 3. airports.csv from OurAirports
echo -e "\n[3/6] Downloading airports.csv..."
curl -fSL "https://davidmegginson.github.io/ourairports-data/airports.csv" -o "$ASSETS_DIR/airports.csv"
echo "  -> airports.csv downloaded"

# 4. airport-frequencies.csv from OurAirports
echo -e "\n[4/6] Downloading airport-frequencies.csv..."
curl -fSL "https://davidmegginson.github.io/ourairports-data/airport-frequencies.csv" -o "$ASSETS_DIR/airport-frequencies.csv"
echo "  -> airport-frequencies.csv downloaded"

# 5. runways.csv from OurAirports
echo -e "\n[5/6] Downloading runways.csv..."
curl -fSL "https://davidmegginson.github.io/ourairports-data/runways.csv" -o "$ASSETS_DIR/runways.csv"
echo "  -> runways.csv downloaded"

# 6. navaids.csv from OurAirports
echo -e "\n[6/6] Downloading navaids.csv..."
curl -fSL "https://davidmegginson.github.io/ourairports-data/navaids.csv" -o "$ASSETS_DIR/navaids.csv"
echo "  -> navaids.csv downloaded"

echo -e "\nAll assets downloaded successfully!"
