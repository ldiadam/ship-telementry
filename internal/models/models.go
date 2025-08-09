package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Vessel struct {
	ID        int64     `json:"id"`
	IMO       *string   `json:"imo"`
	Name      string    `json:"name"`
	Flag      *string   `json:"flag"`
	Type      *string   `json:"type"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Upload struct {
	ID             int64     `json:"id"`
	VesselID       int64     `json:"vessel_id"`
	SourceFilename string    `json:"source_filename"`
	FileHash       string    `json:"file_hash"`
	UploadedAt     time.Time `json:"uploaded_at"`
	Note           *string   `json:"note"`
}

type EngineReading struct {
	ID             int64           `json:"id"`
	VesselID       int64           `json:"vessel_id"`
	EngineNo       *int            `json:"engine_no"`
	Timestamp      time.Time       `json:"ts"`
	RPM            *float64        `json:"rpm"`
	TempC          *float64        `json:"temp_c"`
	OilPressureBar *float64        `json:"oil_pressure_bar"`
	Alarms         *string         `json:"alarms"`
	RowHash        string          `json:"row_hash"`
	ExtraJSON      json.RawMessage `json:"extra_json"`
	CreatedAt      time.Time       `json:"created_at"`
}

type FuelTankReading struct {
	ID           int64           `json:"id"`
	VesselID     int64           `json:"vessel_id"`
	TankNo       *int            `json:"tank_no"`
	Timestamp    time.Time       `json:"ts"`
	LevelPercent *float64        `json:"level_percent"`
	VolumeLiters *float64        `json:"volume_liters"`
	TempC        *float64        `json:"temp_c"`
	RowHash      string          `json:"row_hash"`
	ExtraJSON    json.RawMessage `json:"extra_json"`
	CreatedAt    time.Time       `json:"created_at"`
}

type GeneratorReading struct {
	ID          int64           `json:"id"`
	VesselID    int64           `json:"vessel_id"`
	GenNo       *int            `json:"gen_no"`
	Timestamp   time.Time       `json:"ts"`
	LoadKW      *float64        `json:"load_kw"`
	VoltageV    *float64        `json:"voltage_v"`
	FrequencyHz *float64        `json:"frequency_hz"`
	FuelRateLPH *float64        `json:"fuel_rate_lph"`
	RowHash     string          `json:"row_hash"`
	ExtraJSON   json.RawMessage `json:"extra_json"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CCTVStatusReading struct {
	ID            int64           `json:"id"`
	VesselID      int64           `json:"vessel_id"`
	CamID         *string         `json:"cam_id"`
	Timestamp     time.Time       `json:"ts"`
	Status        *string         `json:"status"`
	UptimePercent *float64        `json:"uptime_percent"`
	RowHash       string          `json:"row_hash"`
	ExtraJSON     json.RawMessage `json:"extra_json"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ImpactVibrationReading struct {
	ID        int64           `json:"id"`
	VesselID  int64           `json:"vessel_id"`
	SensorID  *string         `json:"sensor_id"`
	Timestamp time.Time       `json:"ts"`
	AccelG    *float64        `json:"accel_g"`
	ShockG    *float64        `json:"shock_g"`
	Notes     *string         `json:"notes"`
	RowHash   string          `json:"row_hash"`
	ExtraJSON json.RawMessage `json:"extra_json"`
	CreatedAt time.Time       `json:"created_at"`
}

type LocationReading struct {
	ID            int64           `json:"id"`
	VesselID      int64           `json:"vessel_id"`
	Timestamp     time.Time       `json:"ts"`
	Latitude      *float64        `json:"latitude"`
	Longitude     *float64        `json:"longitude"`
	CourseDegrees *float64        `json:"course_degrees"`
	SpeedKnots    *float64        `json:"speed_knots"`
	Status        *string         `json:"status"`
	RowHash       string          `json:"row_hash"`
	ExtraJSON     json.RawMessage `json:"extra_json"`
	CreatedAt     time.Time       `json:"created_at"`
}

type IngestResponse struct {
	Status       string         `json:"status"`
	UploadID     *int64         `json:"upload_id,omitempty"`
	VesselID     *int64         `json:"vessel_id,omitempty"`
	RowsInserted map[string]int `json:"rows_inserted,omitempty"`
	Warnings     []string       `json:"warnings,omitempty"`
}

type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	NextCursor *string     `json:"next_cursor,omitempty"`
}

// NullString handles nullable string fields
type NullString struct {
	String string
	Valid  bool
}

func (ns *NullString) Scan(value interface{}) error {
	if value == nil {
		ns.String, ns.Valid = "", false
		return nil
	}
	ns.String, ns.Valid = value.(string), true
	return nil
}

func (ns NullString) Value() (driver.Value, error) {
	if !ns.Valid {
		return nil, nil
	}
	return ns.String, nil
}
