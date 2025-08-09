# Vessel Telemetry API

A Golang backend API using Fiber and SQLite to ingest XLSX telemetry files from vessels and provide historical data access.

## Features

- **XLSX Ingestion**: Process Excel files with multiple sheets (Ship Info, Engines, Fuel Tanks, Generators, CCTV, Impact & Vibration)
- **Idempotency**: File-level and row-level deduplication using SHA256 hashing
- **Flexible Mapping**: Fuzzy column name matching with unknown fields stored in JSON
- **Data Validation**: Range validation with configurable warnings
- **Cursor Pagination**: Efficient pagination for large datasets
- **OpenAPI**: Auto-generated API documentation
- **SQLite with WAL**: Optimized for performance with proper indexing

## Tech Stack

- **Go 1.22+**
- **Fiber v2** - Web framework
- **SQLite** with `mattn/go-sqlite3` driver
- **Excelize v2** - XLSX parsing
- **Standard library** for hashing, logging, config

## Quick Start

1. **Install dependencies**:
   ```bash
   go mod tidy
   ```

2. **Set up environment**:
   ```bash
   cp .env.example .env
   # Edit .env if needed
   ```

3. **Run the server**:
   ```bash
   go run ./cmd/server
   ```

4. **Access the web interface**:
   - **Frontend**: http://localhost:8080
   - **API**: http://localhost:8080/vessels
   - **OpenAPI**: http://localhost:8080/.well-known/openapi.json

4. **Create sample data** (optional):
   ```bash
   go run create_sample.go
   ```

5. **Test ingestion**:
   ```bash
   # Preferred: Use IMO number (unique international identifier)
   curl -F "file=@sample_telemetry.xlsx" \
        "http://localhost:8080/ingest/xlsx?imo=9811000&period_start=2025-08-08T10:00:00Z"
   
   # Fallback: Use vessel name if IMO is unknown
   curl -F "file=@sample_telemetry.xlsx" \
        "http://localhost:8080/ingest/xlsx?vessel_name=Ever Given&period_start=2025-08-08T10:00:00Z"
   ```

## Why IMO Numbers?

**IMO (International Maritime Organization) numbers** are the preferred vessel identifier because:

- **Globally unique** - Each vessel has only one IMO number for life
- **Permanent** - Never changes even if vessel name, flag, or owner changes  
- **Standardized** - 7-digit format recognized worldwide (e.g., 9799707)
- **Reliable** - Prevents confusion from duplicate or similar vessel names
- **Industry standard** - Used by port authorities, insurers, and maritime databases

The system **prioritizes IMO** from the query parameter, then falls back to extracting it from the XLSX Ship Info sheet, and finally uses vessel name as a last resort.

## API Endpoints

### Ingestion
- `POST /ingest/xlsx?imo=<imo_number>&period_start=<iso8601>` - Upload XLSX file (preferred)
- `POST /ingest/xlsx?vessel_name=<name>&period_start=<iso8601>` - Upload XLSX file (fallback)

### Vessels
- `GET /vessels` - List all vessels with latest timestamps
- `GET /vessels/:id` - Get vessel details
- `GET /vessels/:id/telemetry?stream=<engines|fuel|generators|cctv|impact|location>` - Get telemetry data
- `GET /vessels/:id/latest?stream=engines&engine_no=1` - Get latest reading

### Uploads
- `GET /uploads/:id` - Get upload details

### Documentation
- `GET /.well-known/openapi.json` - OpenAPI specification

## Configuration

Environment variables (see `.env.example`):

- `PORT=8080` - Server port
- `DB_PATH=./data/telemetry.db` - SQLite database path
- `ALLOW_UNSAFE_DUPLICATE_INGEST=false` - Allow reprocessing same file hash

## Data Model

### Sheets Processed

1. **Ship Info** - Vessel metadata (IMO, name, flag, type) + Location data (GPS coordinates, course, speed, status)
2. **Engines** - RPM, temperature, oil pressure, alarms
3. **Fuel Tanks** - Level %, volume, temperature
4. **Generators** - Load, voltage, frequency, fuel rate
5. **CCTV** - Camera status, uptime
6. **Impact & Vibration** - Acceleration, shock readings

### Column Mapping

The system uses fuzzy matching for column headers:

- **Engines**: `rpm`, `temp`/`temperature`, `oil_pressure`/`pressure`, `alarm`/`alarms`
- **Fuel**: `level`/`level_%`, `volume`/`capacity`, `temp`/`temperature`
- **Generators**: `load`/`load_kw`, `voltage`/`volt`, `frequency`/`freq`, `fuel_rate`
- **CCTV**: `cam_id`/`camera`, `status`, `uptime`/`uptime_percent`
- **Impact**: `sensor_id`/`sensor`, `accel`/`acceleration`, `shock`, `notes`
- **Location**: `latitude`/`lat`, `longitude`/`lon`, `course`/`heading`, `speed`/`speed_knots`, `status`

Unknown columns are stored in the `extra_json` field.

## Pagination

Uses cursor-based pagination for efficient large dataset traversal:

```bash
# First page
GET /vessels/1/telemetry?stream=engines&limit=100

# Next page using returned cursor
GET /vessels/1/telemetry?stream=engines&limit=100&cursor=<base64_cursor>
```

## Data Validation

- **Engines**: RPM ≥ 0, oil pressure ≥ 0
- **Fuel**: Level 0-100%, volume ≥ 0
- **Generators**: Load ≥ 0, voltage ≥ 0, frequency 45-70 Hz, fuel rate ≥ 0

Invalid rows are skipped with warnings in the response.

## Idempotency

- **File-level**: SHA256 hash of entire XLSX prevents reprocessing
- **Row-level**: Unique constraint on `(vessel_id, ts, row_hash)` prevents duplicates

## Testing

Run tests:
```bash
go test ./...
```

Tests cover:
- Hash functions and row hashing
- Column mapping and parsing
- Pagination encoding/decoding
- Data validation

## Database Schema

SQLite with WAL mode enabled. Key tables:

- `vessels` - Ship metadata
- `uploads` - File tracking with hashes
- `*_readings` - Time-series data (engines, fuel, generators, cctv, impact)
- `vessel_stream_latest` - Latest timestamp per stream for quick access

## Performance

- WAL mode with optimized pragmas
- Proper indexing on `(vessel_id, ts)`
- Cursor pagination for large datasets
- `INSERT OR IGNORE` for efficient deduplication

## Error Handling

- `400` - Missing parameters or invalid format
- `409` - Duplicate file when `ALLOW_UNSAFE_DUPLICATE_INGEST=false`
- `422` - Invalid data (warnings returned, valid rows still processed)
- `500` - Internal server errors

## Example Response

```json
{
  "status": "ingested",
  "upload_id": 123,
  "vessel_id": 7,
  "rows_inserted": {
    "engines": 120,
    "fuel": 30,
    "generators": 15,
    "cctv": 10,
    "impact": 6
  },
  "warnings": [
    "row 17 engines: negative rpm skipped"
  ]
}
```