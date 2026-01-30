# Static Assets

This directory contains static data files used by Co-ATC for metadata enrichment and AI context.

## Data Sources

### `aircraft.csv`
Database of aircraft metadata (registration, type code, manufacturer, etc.) used to enrich live ADS-B data, particularly for sources that do not provide full metadata (e.g., OpenSky).

*   **Source**: [wiedehopf/tar1090-db](https://github.com/wiedehopf/tar1090-db)
*   **Format**: Semicolon-separated CSV (Hex;Registration;Type;...)

### `airlines.dat`
Database of airline operators including ICAO/IATA codes, callsigns, and country of origin. used for flight number matching and operator identification.

*   **Source**: [OpenFlights.org](https://openflights.org/data)
*   **Format**: CSV (ID, Name, Alias, IATA, ICAO, Callsign, Country, Active)

### `airports.csv`
Database of airport locations and attributes used to automatically configure station coordinates and elevation.

*   **Source**: [OurAirports.com](https://ourairports.com/data/)
*   **Format**: CSV (id, ident, type, name, latitude_deg, longitude_deg, elevation_ft, ...)

### `runways.csv`
Runway threshold coordinates for supported airports, used for accurate landing and takeoff detection relative to specific runways.

*   **Source**: [OurAirports.com](https://ourairports.com/data/)
*   **Format**: CSV (id, airport_ident, le_ident, le_latitude_deg, le_longitude_deg, ...)

### `recat_eu_aircraft.csv`
Database of aircraft Wake Turbulence Categories (RECAT-EU) based on ICAO aircraft type designators. Used to infer wake turbulence category from aircraft type.

*   **Source**: [EASA - Assignment of ICAO Aircraft Types to RECAT-EU Wake Turbulence Categories](https://www.easa.europa.eu/en/assignment-icao-aircraft-types-recat-eu-wake-turbulence-categories)
*   **Format**: CSV (Manufacturer, Model, ICAO Type Designator, ICAO Legacy WTC, RECAT-EU WTC)

#### Updating `recat_eu_aircraft.csv`

The CSV is generated from the official EASA PDF using a Python script.

**Prerequisites:**
- [uv](https://github.com/astral-sh/uv) (fast Python package installer and resolver)

**Instructions:**
1.  Download the latest PDF from the EASA website and save it as `assets/extract-recat-eu/recat-eu.pdf`.
2.  Navigate to `assets/extract-recat-eu`.
3.  Run the script:
    ```bash
    uv run main.py
    ```
4.  The `recat_eu_aircraft.csv` file will be generated in the same directory.

-----
Copy/pasted from: https://github.com/stignarnia/co-atc/blob/main/assets/README.md