-- Enable SQLite optimizations
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;
PRAGMA cache_size=-20000;

-- vessels (from "Ship Info")
CREATE TABLE IF NOT EXISTS vessels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    imo TEXT UNIQUE,            -- nullable if unknown
    name TEXT,
    flag TEXT,
    type TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    updated_at DATETIME DEFAULT (datetime('now'))
);

-- uploads (one per XLSX)
CREATE TABLE IF NOT EXISTS uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    source_filename TEXT,
    file_hash TEXT UNIQUE NOT NULL,
    uploaded_at DATETIME NOT NULL,  -- server receive time
    note TEXT,
    FOREIGN KEY(vessel_id) REFERENCES vessels(id)
);

-- Generic pattern for time-series tables:
-- Common columns: id, vessel_id, ts, row_hash, extra_json, created_at
-- Add domain fields as needed.

CREATE TABLE IF NOT EXISTS engine_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    engine_no INTEGER,          -- 1..N
    ts DATETIME NOT NULL,
    rpm REAL,                   -- >= 0
    temp_c REAL,
    oil_pressure_bar REAL,
    alarms TEXT,
    row_hash TEXT NOT NULL,
    extra_json TEXT,            -- JSON dump of unmapped cols
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_engine_ts ON engine_readings(vessel_id, ts);

CREATE TABLE IF NOT EXISTS fuel_tank_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    tank_no TEXT,
    ts DATETIME NOT NULL,
    level_percent REAL,          -- 0..100
    volume_liters REAL,          -- >= 0
    temp_c REAL,
    row_hash TEXT NOT NULL,
    extra_json TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_fuel_ts ON fuel_tank_readings(vessel_id, ts);

CREATE TABLE IF NOT EXISTS generator_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    gen_no INTEGER,
    ts DATETIME NOT NULL,
    load_kw REAL,
    voltage_v REAL,
    frequency_hz REAL,
    fuel_rate_lph REAL,
    row_hash TEXT NOT NULL,
    extra_json TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_gen_ts ON generator_readings(vessel_id, ts);

CREATE TABLE IF NOT EXISTS cctv_status_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    cam_id TEXT,
    ts DATETIME NOT NULL,
    status TEXT,               -- e.g., OK, OFFLINE
    uptime_percent REAL,
    row_hash TEXT NOT NULL,
    extra_json TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_cctv_ts ON cctv_status_readings(vessel_id, ts);

CREATE TABLE IF NOT EXISTS impact_vibration_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    sensor_id TEXT,
    ts DATETIME NOT NULL,
    accel_g REAL,
    shock_g REAL,
    notes TEXT,
    row_hash TEXT NOT NULL,
    extra_json TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_imp_ts ON impact_vibration_readings(vessel_id, ts);

CREATE TABLE IF NOT EXISTS location_readings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vessel_id INTEGER NOT NULL,
    ts DATETIME NOT NULL,
    latitude REAL,              -- -90 to 90
    longitude REAL,             -- -180 to 180
    course_degrees REAL,        -- 0-360
    speed_knots REAL,           -- >= 0
    status TEXT,                -- underway, anchored, moored, etc.
    row_hash TEXT NOT NULL,
    extra_json TEXT,
    created_at DATETIME DEFAULT (datetime('now')),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id),
    UNIQUE(vessel_id, ts, row_hash)
);

CREATE INDEX IF NOT EXISTS idx_location_ts ON location_readings(vessel_id, ts);

-- lightweight materialized view for "latest timestamp per stream"
CREATE TABLE IF NOT EXISTS vessel_stream_latest (
    vessel_id INTEGER NOT NULL,
    stream TEXT NOT NULL,       -- engines|fuel|generators|cctv|impact|location
    latest_ts DATETIME NOT NULL,
    PRIMARY KEY (vessel_id, stream),
    FOREIGN KEY(vessel_id) REFERENCES vessels(id)
);