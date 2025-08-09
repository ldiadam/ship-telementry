package api

import (
	"database/sql"
	"io"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"

	"vessel-telemetry-api/internal/ingest"
	"vessel-telemetry-api/internal/models"
)

type Handlers struct {
	db                         *sql.DB
	processor                  *ingest.XLSXProcessor
	allowUnsafeDuplicateIngest bool
}

func NewHandlers(db *sql.DB, allowUnsafeDuplicateIngest bool) *Handlers {
	return &Handlers{
		db:                         db,
		processor:                  ingest.NewXLSXProcessor(db, allowUnsafeDuplicateIngest),
		allowUnsafeDuplicateIngest: allowUnsafeDuplicateIngest,
	}
}

// GetHealthz provides a health check endpoint for Docker deployments
func (h *Handlers) GetHealthz(c *fiber.Ctx) error {
	// Check database connectivity
	if err := h.db.Ping(); err != nil {
		return c.Status(503).JSON(fiber.Map{
			"status":  "unhealthy",
			"error":   "database connection failed",
			"details": err.Error(),
		})
	}

	// Check if we can query the database
	var count int
	err := h.db.QueryRow("SELECT COUNT(*) FROM vessels").Scan(&count)
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"status":  "unhealthy",
			"error":   "database query failed",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"database":  "connected",
		"vessels":   count,
	})
}

func (h *Handlers) PostIngestXLSX(c *fiber.Ctx) error {
	// Primary: Use IMO if provided
	imo := c.Query("imo")

	// Fallback: Use vessel_name (for backwards compatibility or when IMO is unknown)
	vesselName := c.Query("vessel_name")

	// At least one identifier is required
	if imo == "" && vesselName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "either 'imo' or 'vessel_name' parameter is required"})
	}

	var periodStart *time.Time
	if periodStartStr := c.Query("period_start"); periodStartStr != "" {
		if ts, err := time.Parse(time.RFC3339, periodStartStr); err == nil {
			periodStart = &ts
		} else {
			return c.Status(400).JSON(fiber.Map{"error": "invalid period_start format, use ISO 8601"})
		}
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "file is required"})
	}

	// Read file data
	fileReader, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to open file"})
	}
	defer fileReader.Close()

	fileData, err := io.ReadAll(fileReader)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "failed to read file"})
	}

	// Process file - pass both IMO and vessel name, processor will prioritize IMO
	response, err := h.processor.ProcessFile(fileData, file.Filename, imo, vesselName, periodStart)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if response.Status == "already_ingested" {
		if !h.allowUnsafeDuplicateIngest {
			return c.Status(409).JSON(response)
		}
	}

	return c.JSON(response)
}

func (h *Handlers) GetVessels(c *fiber.Ctx) error {
	query := `
		SELECT v.id, v.imo, v.name, v.flag, v.type, v.created_at, v.updated_at
		FROM vessels v
		ORDER BY v.name
	`

	rows, err := h.db.Query(query)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var vessels []map[string]interface{}

	for rows.Next() {
		var vessel models.Vessel
		var imo, flag, vesselType sql.NullString

		err := rows.Scan(
			&vessel.ID, &imo, &vessel.Name, &flag, &vesselType,
			&vessel.CreatedAt, &vessel.UpdatedAt,
		)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		if imo.Valid {
			vessel.IMO = &imo.String
		}
		if flag.Valid {
			vessel.Flag = &flag.String
		}
		if vesselType.Valid {
			vessel.Type = &vesselType.String
		}

		// Get latest timestamps per stream
		latestQuery := `
			SELECT stream, latest_ts 
			FROM vessel_stream_latest 
			WHERE vessel_id = ?
		`
		latestRows, err := h.db.Query(latestQuery, vessel.ID)
		if err == nil {
			latest := make(map[string]time.Time)
			for latestRows.Next() {
				var stream string
				var ts time.Time
				if err := latestRows.Scan(&stream, &ts); err == nil {
					latest[stream] = ts
				}
			}
			latestRows.Close()

			vesselMap := map[string]interface{}{
				"id":         vessel.ID,
				"imo":        vessel.IMO,
				"name":       vessel.Name,
				"flag":       vessel.Flag,
				"type":       vessel.Type,
				"created_at": vessel.CreatedAt,
				"updated_at": vessel.UpdatedAt,
				"latest":     latest,
			}
			vessels = append(vessels, vesselMap)
		}
	}

	return c.JSON(vessels)
}

func (h *Handlers) GetVessel(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid vessel id"})
	}

	query := `
		SELECT id, imo, name, flag, type, created_at, updated_at
		FROM vessels 
		WHERE id = ?
	`

	var vessel models.Vessel
	var imo, flag, vesselType sql.NullString

	err = h.db.QueryRow(query, id).Scan(
		&vessel.ID, &imo, &vessel.Name, &flag, &vesselType,
		&vessel.CreatedAt, &vessel.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "vessel not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if imo.Valid {
		vessel.IMO = &imo.String
	}
	if flag.Valid {
		vessel.Flag = &flag.String
	}
	if vesselType.Valid {
		vessel.Type = &vesselType.String
	}

	// Get latest timestamps per stream
	latestQuery := `
		SELECT stream, latest_ts 
		FROM vessel_stream_latest 
		WHERE vessel_id = ?
	`
	latestRows, err := h.db.Query(latestQuery, id)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer latestRows.Close()

	latest := make(map[string]time.Time)
	for latestRows.Next() {
		var stream string
		var ts time.Time
		if err := latestRows.Scan(&stream, &ts); err == nil {
			latest[stream] = ts
		}
	}

	response := map[string]interface{}{
		"id":         vessel.ID,
		"imo":        vessel.IMO,
		"name":       vessel.Name,
		"flag":       vessel.Flag,
		"type":       vessel.Type,
		"created_at": vessel.CreatedAt,
		"updated_at": vessel.UpdatedAt,
		"latest":     latest,
	}

	return c.JSON(response)
}

func (h *Handlers) GetVesselTelemetry(c *fiber.Ctx) error {
	vesselID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid vessel id"})
	}

	stream := c.Query("stream")
	if stream == "" {
		return c.Status(400).JSON(fiber.Map{"error": "stream parameter is required"})
	}

	limit := 200
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	cursor := c.Query("cursor")
	cursorTS, cursorID, err := DecodeCursor(cursor)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid cursor"})
	}

	var query string
	var args []interface{}

	switch stream {
	case "engines":
		query = `
			SELECT id, vessel_id, engine_no, ts, rpm, temp_c, oil_pressure_bar, alarms, row_hash, extra_json, created_at
			FROM engine_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if engineNoStr := c.Query("engine_no"); engineNoStr != "" {
			if engineNo, err := strconv.Atoi(engineNoStr); err == nil {
				query += " AND engine_no = ?"
				args = append(args, engineNo)
			}
		}

	case "fuel":
		query = `
			SELECT id, vessel_id, tank_no, ts, level_percent, volume_liters, temp_c, row_hash, extra_json, created_at
			FROM fuel_tank_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if tankNoStr := c.Query("tank_no"); tankNoStr != "" {
			if tankNo, err := strconv.Atoi(tankNoStr); err == nil {
				query += " AND tank_no = ?"
				args = append(args, tankNo)
			}
		}

	case "generators":
		query = `
			SELECT id, vessel_id, gen_no, ts, load_kw, voltage_v, frequency_hz, fuel_rate_lph, row_hash, extra_json, created_at
			FROM generator_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if genNoStr := c.Query("gen_no"); genNoStr != "" {
			if genNo, err := strconv.Atoi(genNoStr); err == nil {
				query += " AND gen_no = ?"
				args = append(args, genNo)
			}
		}

	case "cctv":
		query = `
			SELECT id, vessel_id, cam_id, ts, status, uptime_percent, row_hash, extra_json, created_at
			FROM cctv_status_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if camID := c.Query("cam_id"); camID != "" {
			query += " AND cam_id = ?"
			args = append(args, camID)
		}

	case "impact":
		query = `
			SELECT id, vessel_id, sensor_id, ts, accel_g, shock_g, notes, row_hash, extra_json, created_at
			FROM impact_vibration_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if sensorID := c.Query("sensor_id"); sensorID != "" {
			query += " AND sensor_id = ?"
			args = append(args, sensorID)
		}

	case "location":
		query = `
			SELECT id, vessel_id, ts, latitude, longitude, course_degrees, speed_knots, status, row_hash, extra_json, created_at
			FROM location_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

	default:
		return c.Status(400).JSON(fiber.Map{"error": "invalid stream"})
	}

	// Add time range filters
	if from := c.Query("from"); from != "" {
		if fromTime, err := time.Parse(time.RFC3339, from); err == nil {
			query += " AND ts >= ?"
			args = append(args, fromTime)
		}
	}

	if to := c.Query("to"); to != "" {
		if toTime, err := time.Parse(time.RFC3339, to); err == nil {
			query += " AND ts <= ?"
			args = append(args, toTime)
		}
	}

	// Add cursor pagination
	if !cursorTS.IsZero() {
		query += " AND (ts > ? OR (ts = ? AND id > ?))"
		args = append(args, cursorTS, cursorTS, cursorID)
	}

	query += " ORDER BY ts, id LIMIT ?"
	args = append(args, limit+1) // Get one extra to check if there's a next page

	rows, err := h.db.Query(query, args...)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	var items []interface{}
	var lastTS time.Time
	var lastID int64

	count := 0
	for rows.Next() && count < limit {
		count++

		switch stream {
		case "engines":
			var reading models.EngineReading
			var engineNo sql.NullInt64
			var rpm, tempC, oilPressure sql.NullFloat64
			var alarms sql.NullString

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &engineNo, &reading.Timestamp,
				&rpm, &tempC, &oilPressure, &alarms,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if engineNo.Valid {
				val := int(engineNo.Int64)
				reading.EngineNo = &val
			}
			if rpm.Valid {
				reading.RPM = &rpm.Float64
			}
			if tempC.Valid {
				reading.TempC = &tempC.Float64
			}
			if oilPressure.Valid {
				reading.OilPressureBar = &oilPressure.Float64
			}
			if alarms.Valid {
				reading.Alarms = &alarms.String
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID

		case "fuel":
			var reading models.FuelTankReading
			var tankNo sql.NullInt64
			var level, volume, tempC sql.NullFloat64

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &tankNo, &reading.Timestamp,
				&level, &volume, &tempC,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if tankNo.Valid {
				val := int(tankNo.Int64)
				reading.TankNo = &val
			}
			if level.Valid {
				reading.LevelPercent = &level.Float64
			}
			if volume.Valid {
				reading.VolumeLiters = &volume.Float64
			}
			if tempC.Valid {
				reading.TempC = &tempC.Float64
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID

		case "generators":
			var reading models.GeneratorReading
			var genNo sql.NullInt64
			var loadKW, voltageV, frequencyHz, fuelRateLPH sql.NullFloat64

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &genNo, &reading.Timestamp,
				&loadKW, &voltageV, &frequencyHz, &fuelRateLPH,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if genNo.Valid {
				val := int(genNo.Int64)
				reading.GenNo = &val
			}
			if loadKW.Valid {
				reading.LoadKW = &loadKW.Float64
			}
			if voltageV.Valid {
				reading.VoltageV = &voltageV.Float64
			}
			if frequencyHz.Valid {
				reading.FrequencyHz = &frequencyHz.Float64
			}
			if fuelRateLPH.Valid {
				reading.FuelRateLPH = &fuelRateLPH.Float64
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID

		case "cctv":
			var reading models.CCTVStatusReading
			var camID, status sql.NullString
			var uptimePercent sql.NullFloat64

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &camID, &reading.Timestamp,
				&status, &uptimePercent,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if camID.Valid {
				reading.CamID = &camID.String
			}
			if status.Valid {
				reading.Status = &status.String
			}
			if uptimePercent.Valid {
				reading.UptimePercent = &uptimePercent.Float64
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID

		case "impact":
			var reading models.ImpactVibrationReading
			var sensorID, notes sql.NullString
			var accelG, shockG sql.NullFloat64

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &sensorID, &reading.Timestamp,
				&accelG, &shockG, &notes,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if sensorID.Valid {
				reading.SensorID = &sensorID.String
			}
			if accelG.Valid {
				reading.AccelG = &accelG.Float64
			}
			if shockG.Valid {
				reading.ShockG = &shockG.Float64
			}
			if notes.Valid {
				reading.Notes = &notes.String
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID

		case "location":
			var reading models.LocationReading
			var latitude, longitude, course, speed sql.NullFloat64
			var status sql.NullString

			err := rows.Scan(
				&reading.ID, &reading.VesselID, &reading.Timestamp,
				&latitude, &longitude, &course, &speed, &status,
				&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
			)
			if err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}

			if latitude.Valid {
				reading.Latitude = &latitude.Float64
			}
			if longitude.Valid {
				reading.Longitude = &longitude.Float64
			}
			if course.Valid {
				reading.CourseDegrees = &course.Float64
			}
			if speed.Valid {
				reading.SpeedKnots = &speed.Float64
			}
			if status.Valid {
				reading.Status = &status.String
			}

			items = append(items, reading)
			lastTS = reading.Timestamp
			lastID = reading.ID
		}
	}

	response := models.PaginatedResponse{
		Items: items,
	}

	// Check if there's a next page
	if rows.Next() {
		nextCursor := EncodeCursor(lastTS, lastID)
		response.NextCursor = &nextCursor
	}

	return c.JSON(response)
}

func (h *Handlers) GetVesselLatest(c *fiber.Ctx) error {
	vesselID, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid vessel id"})
	}

	stream := c.Query("stream")
	if stream == "" {
		return c.Status(400).JSON(fiber.Map{"error": "stream parameter is required"})
	}

	var query string
	var args []interface{}

	switch stream {
	case "engines":
		query = `
			SELECT id, vessel_id, engine_no, ts, rpm, temp_c, oil_pressure_bar, alarms, row_hash, extra_json, created_at
			FROM engine_readings 
			WHERE vessel_id = ?
		`
		args = append(args, vesselID)

		if engineNoStr := c.Query("engine_no"); engineNoStr != "" {
			if engineNo, err := strconv.Atoi(engineNoStr); err == nil {
				query += " AND engine_no = ?"
				args = append(args, engineNo)
			}
		}

		query += " ORDER BY ts DESC, id DESC LIMIT 1"

		var reading models.EngineReading
		var engineNo sql.NullInt64
		var rpm, tempC, oilPressure sql.NullFloat64
		var alarms sql.NullString

		err := h.db.QueryRow(query, args...).Scan(
			&reading.ID, &reading.VesselID, &engineNo, &reading.Timestamp,
			&rpm, &tempC, &oilPressure, &alarms,
			&reading.RowHash, &reading.ExtraJSON, &reading.CreatedAt,
		)
		if err == sql.ErrNoRows {
			return c.Status(404).JSON(fiber.Map{"error": "no data found"})
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}

		if engineNo.Valid {
			val := int(engineNo.Int64)
			reading.EngineNo = &val
		}
		if rpm.Valid {
			reading.RPM = &rpm.Float64
		}
		if tempC.Valid {
			reading.TempC = &tempC.Float64
		}
		if oilPressure.Valid {
			reading.OilPressureBar = &oilPressure.Float64
		}
		if alarms.Valid {
			reading.Alarms = &alarms.String
		}

		return c.JSON(reading)

	default:
		return c.Status(400).JSON(fiber.Map{"error": "stream not implemented for latest endpoint"})
	}
}

func (h *Handlers) GetUpload(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "invalid upload id"})
	}

	query := `
		SELECT id, vessel_id, source_filename, file_hash, uploaded_at, note
		FROM uploads 
		WHERE id = ?
	`

	var upload models.Upload
	var note sql.NullString

	err = h.db.QueryRow(query, id).Scan(
		&upload.ID, &upload.VesselID, &upload.SourceFilename,
		&upload.FileHash, &upload.UploadedAt, &note,
	)
	if err == sql.ErrNoRows {
		return c.Status(404).JSON(fiber.Map{"error": "upload not found"})
	}
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	if note.Valid {
		upload.Note = &note.String
	}

	return c.JSON(upload)
}

func (h *Handlers) GetOpenAPI(c *fiber.Ctx) error {
	openAPISpec := map[string]interface{}{
		"openapi": "3.0.0",
		"info": map[string]interface{}{
			"title":   "Vessel Telemetry API",
			"version": "1.0.0",
		},
		"paths": map[string]interface{}{
			"/ingest/xlsx": map[string]interface{}{
				"post": map[string]interface{}{
					"summary": "Ingest XLSX telemetry file",
					"parameters": []map[string]interface{}{
						{
							"name":     "vessel_name",
							"in":       "query",
							"required": true,
							"schema":   map[string]string{"type": "string"},
						},
						{
							"name":     "period_start",
							"in":       "query",
							"required": false,
							"schema":   map[string]string{"type": "string", "format": "date-time"},
						},
					},
					"requestBody": map[string]interface{}{
						"content": map[string]interface{}{
							"multipart/form-data": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"file": map[string]interface{}{
											"type":   "string",
											"format": "binary",
										},
									},
								},
							},
						},
					},
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
			"/vessels": map[string]interface{}{
				"get": map[string]interface{}{
					"summary": "List vessels",
					"responses": map[string]interface{}{
						"200": map[string]interface{}{
							"description": "Success",
						},
					},
				},
			},
		},
	}

	return c.JSON(openAPISpec)
}
