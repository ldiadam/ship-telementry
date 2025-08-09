package ingest

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"

	"vessel-telemetry-api/internal/models"
	"vessel-telemetry-api/internal/util"
)

type XLSXProcessor struct {
	db                         *sql.DB
	allowUnsafeDuplicateIngest bool
}

func NewXLSXProcessor(db *sql.DB, allowUnsafeDuplicateIngest bool) *XLSXProcessor {
	return &XLSXProcessor{
		db:                         db,
		allowUnsafeDuplicateIngest: allowUnsafeDuplicateIngest,
	}
}

func (p *XLSXProcessor) ProcessFile(fileData []byte, filename, imo, vesselName string, periodStart *time.Time) (*models.IngestResponse, error) {
	// Compute file hash
	fileHash := util.SHA256Hex(fileData)

	// Check if already processed
	var existingUploadID int64
	err := p.db.QueryRow("SELECT id FROM uploads WHERE file_hash = ?", fileHash).Scan(&existingUploadID)
	if err == nil {
		return &models.IngestResponse{
			Status:   "already_ingested",
			UploadID: &existingUploadID,
		}, nil
	} else if err != sql.ErrNoRows {
		return nil, fmt.Errorf("error checking file hash: %w", err)
	}

	// Parse XLSX
	f, err := excelize.OpenReader(strings.NewReader(string(fileData)))
	if err != nil {
		return nil, fmt.Errorf("error opening XLSX: %w", err)
	}
	defer f.Close()

	uploadedAt := time.Now()
	if periodStart != nil {
		uploadedAt = *periodStart
	}

	// Process Ship Info sheet first
	vesselID, locationCount, locationWarnings, err := p.processShipInfo(f, imo, vesselName, uploadedAt)
	if err != nil {
		return nil, fmt.Errorf("error processing ship info: %w", err)
	}

	// Create upload record
	//result, err := p.db.Exec(
	//	"INSERT INTO uploads (vessel_id, source_filename, file_hash, uploaded_at) VALUES (?, ?, ?, ?)",
	//	vesselID, filename, fileHash, uploadedAt,
	//)
	//if err != nil {
	//	return nil, fmt.Errorf("error creating upload record: %w", err)
	//}

	//uploadID, _ := result.LastInsertId()
	uploadID := int64(1)

	// Process telemetry sheets
	rowsInserted := make(map[string]int)
	var warnings []string

	// Add location data from Ship Info processing
	if locationCount > 0 {
		rowsInserted["location"] = locationCount
	}
	warnings = append(warnings, locationWarnings...)

	sheets := f.GetSheetList()
	for _, sheetName := range sheets {
		sheetNameLower := strings.ToLower(sheetName)

		switch {
		case strings.Contains(sheetNameLower, "engine"):
			count, warns := p.processEngineSheet(f, sheetName, vesselID, uploadedAt)
			rowsInserted["engines"] = count
			warnings = append(warnings, warns...)
		case strings.Contains(sheetNameLower, "fuel"):
			count, warns := p.processFuelSheet(f, sheetName, vesselID, uploadedAt)
			rowsInserted["fuel"] = count
			warnings = append(warnings, warns...)
		case strings.Contains(sheetNameLower, "generator"):
			count, warns := p.processGeneratorSheet(f, sheetName, vesselID, uploadedAt)
			rowsInserted["generators"] = count
			warnings = append(warnings, warns...)
		case strings.Contains(sheetNameLower, "cctv"):
			count, warns := p.processCCTVSheet(f, sheetName, vesselID, uploadedAt)
			rowsInserted["cctv"] = count
			warnings = append(warnings, warns...)
		case strings.Contains(sheetNameLower, "impact") || strings.Contains(sheetNameLower, "vibration"):
			count, warns := p.processImpactSheet(f, sheetName, vesselID, uploadedAt)
			rowsInserted["impact"] = count
			warnings = append(warnings, warns...)
		}
	}

	// Update vessel_stream_latest
	p.updateStreamLatest(vesselID, rowsInserted, uploadedAt)

	return &models.IngestResponse{
		Status:       "ingested",
		UploadID:     &uploadID,
		VesselID:     &vesselID,
		RowsInserted: rowsInserted,
		Warnings:     warnings,
	}, nil
}

func (p *XLSXProcessor) processShipInfo(f *excelize.File, providedIMO, vesselName string, uploadedAt time.Time) (int64, int, []string, error) {
	sheets := f.GetSheetList()
	var shipInfoSheet string

	for _, sheet := range sheets {
		if strings.Contains(strings.ToLower(sheet), "ship") && strings.Contains(strings.ToLower(sheet), "info") {
			shipInfoSheet = sheet
			break
		}
	}

	if shipInfoSheet == "" {
		// No ship info sheet, create vessel with provided identifiers
		if providedIMO != "" {
			// Use provided vessel name or default to IMO-based name
			name := vesselName
			if name == "" {
				name = fmt.Sprintf("Vessel-%s", providedIMO)
			}
			result, err := p.db.Exec("INSERT INTO vessels (imo, name) VALUES (?, ?)", providedIMO, name)
			if err != nil {
				return 0, 0, nil, err
			}
			id, _ := result.LastInsertId()
			return id, 0, nil, nil
		} else {
			if vesselName == "" {
				return 0, 0, nil, fmt.Errorf("vessel name is required when IMO is not provided")
			}
			result, err := p.db.Exec("INSERT INTO vessels (name) VALUES (?)", vesselName)
			if err != nil {
				return 0, 0, nil, err
			}
			id, _ := result.LastInsertId()
			return id, 0, nil, nil
		}
	}

	rows, err := f.GetRows(shipInfoSheet)
	if err != nil || len(rows) < 2 {
		// Fallback to provided identifiers
		if providedIMO != "" {
			// Use provided vessel name or default to IMO-based name
			name := vesselName
			if name == "" {
				name = fmt.Sprintf("Vessel-%s", providedIMO)
			}
			result, err := p.db.Exec("INSERT INTO vessels (imo, name) VALUES (?, ?)", providedIMO, name)
			if err != nil {
				return 0, 0, nil, err
			}
			id, _ := result.LastInsertId()
			return id, 0, nil, nil
		} else {
			if vesselName == "" {
				return 0, 0, nil, fmt.Errorf("vessel name is required when IMO is not provided")
			}
			result, err := p.db.Exec("INSERT INTO vessels (name) VALUES (?)", vesselName)
			if err != nil {
				return 0, 0, nil, err
			}
			id, _ := result.LastInsertId()
			return id, 0, nil, nil
		}
	}

	headers := rows[0]
	data := rows[1]

	mapper := NewHeaderMapper(headers)

	var imo, name, flag, vesselType *string

	// Prioritize provided IMO over extracted IMO
	if providedIMO != "" {
		imo = &providedIMO
	} else if imoCol, found := mapper.FindHeader("imo"); found {
		for i, h := range headers {
			if h == imoCol && i < len(data) && data[i] != "" {
				val := data[i]
				imo = &val
				break
			}
		}
	}

	if nameCol, found := mapper.FindHeader("name", "vessel_name", "ship_name"); found {
		for i, h := range headers {
			if h == nameCol && i < len(data) && data[i] != "" {
				val := data[i]
				name = &val
				break
			}
		}
	}

	if flagCol, found := mapper.FindHeader("flag"); found {
		for i, h := range headers {
			if h == flagCol && i < len(data) && data[i] != "" {
				val := data[i]
				flag = &val
				break
			}
		}
	}

	if typeCol, found := mapper.FindHeader("type", "vessel_type", "ship_type"); found {
		for i, h := range headers {
			if h == typeCol && i < len(data) && data[i] != "" {
				val := data[i]
				vesselType = &val
				break
			}
		}
	}

	if name == nil {
		if vesselName != "" {
			name = &vesselName
		} else if imo != nil {
			// Generate a default name based on IMO when no name is provided
			defaultName := fmt.Sprintf("Vessel-%s", *imo)
			name = &defaultName
		}
	}

	var vesselID int64

	// Try to find existing vessel by IMO or name
	if imo != nil {
		var existingID int64
		err := p.db.QueryRow("SELECT id FROM vessels WHERE imo = ?", *imo).Scan(&existingID)
		if err == nil {
			// Update existing vessel
			_, err = p.db.Exec(
				"UPDATE vessels SET name = ?, flag = ?, type = ?, updated_at = datetime('now') WHERE id = ?",
				*name, flag, vesselType, existingID,
			)
			if err != nil {
				return 0, 0, nil, err
			}
			vesselID = existingID
		}
	}

	if vesselID == 0 {
		// Create new vessel
		result, err := p.db.Exec(
			"INSERT INTO vessels (imo, name, flag, type) VALUES (?, ?, ?, ?)",
			imo, *name, flag, vesselType,
		)
		if err != nil {
			return 0, 0, nil, err
		}
		vesselID, _ = result.LastInsertId()
	}

	// Process location data from Ship Info sheet
	locationCount, locationWarnings := p.processLocationFromShipInfo(headers, data, vesselID, uploadedAt, mapper)

	return vesselID, locationCount, locationWarnings, nil
}

func (p *XLSXProcessor) processEngineSheet(f *excelize.File, sheetName string, vesselID int64, defaultTS time.Time) (int, []string) {
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return 0, []string{fmt.Sprintf("error reading %s sheet", sheetName)}
	}

	headers := rows[0]
	mapper := NewHeaderMapper(headers)

	var warnings []string
	inserted := 0

	tsCol, hasTS := mapper.FindTimestampHeader()
	if hasTS {
		fmt.Printf("DEBUG: Found timestamp column '%s' in engine sheet\n", tsCol)
	} else {
		fmt.Printf("DEBUG: No timestamp column found in engine sheet. Available headers: %v\n", headers)
	}
	engineNoCol, _ := mapper.FindHeader("engine_no", "engine", "eng_no")
	rpmCol, _ := mapper.FindHeader("rpm")
	tempCol, _ := mapper.FindHeader("temp", "temperature", "temp_c")
	pressureCol, _ := mapper.FindHeader("oil_pressure", "pressure", "oil_press")
	alarmsCol, _ := mapper.FindHeader("alarm", "alarms", "alert")

	mappedCols := []string{tsCol, engineNoCol, rpmCol, tempCol, pressureCol, alarmsCol}

	for i := 1; i < len(rows); i++ {
		row := make(map[string]string)
		for j, cell := range rows[i] {
			if j < len(headers) {
				row[headers[j]] = cell
			}
		}

		// Parse timestamp
		ts := defaultTS
		if hasTS && tsCol != "" {
			if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
				ts = parsedTS
			}
		}

		// Parse fields
		var engineNo *int
		var rpm, tempC, oilPressure *float64
		var alarms *string

		var numOnly = regexp.MustCompile(`\d+`)

		if engineNoCol != "" {
			match := numOnly.FindString(row[engineNoCol]) // extract only digits
			if match != "" {
				if val, err := strconv.Atoi(match); err == nil {
					engineNo = &val // assign pointer to the parsed int
				}
			}
		}
		if rpmCol != "" {
			rpm, _ = ParseFloat(row[rpmCol])
		}
		if tempCol != "" {
			tempC, _ = ParseFloat(row[tempCol])
		}
		if pressureCol != "" {
			oilPressure, _ = ParseFloat(row[pressureCol])
		}
		if alarmsCol != "" && row[alarmsCol] != "" {
			val := row[alarmsCol]
			alarms = &val
		}

		// Validate
		if warns := ValidateEngineData(rpm, tempC, oilPressure); len(warns) > 0 {
			warnings = append(warnings, fmt.Sprintf("row %d engines: %s", i+1, strings.Join(warns, ", ")))
			continue
		}

		// Build extra JSON
		extraJSON, _ := BuildExtraJSON(row, mappedCols)

		// Create row hash
		hashKeys := []string{}
		if engineNo != nil {
			hashKeys = append(hashKeys, fmt.Sprintf("engine_no:%d", *engineNo))
		}
		hashKeys = append(hashKeys, string(extraJSON))
		rowHash := util.HashRow(vesselID, ts, "engines", hashKeys...)

		// Insert
		_, err := p.db.Exec(`
			INSERT OR IGNORE INTO engine_readings 
			(vessel_id, engine_no, ts, rpm, temp_c, oil_pressure_bar, alarms, row_hash, extra_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			vesselID, engineNo, ts, rpm, tempC, oilPressure, alarms, rowHash, extraJSON,
		)
		if err == nil {
			inserted++
		}
	}

	return inserted, warnings
}

func (p *XLSXProcessor) processFuelSheet(f *excelize.File, sheetName string, vesselID int64, defaultTS time.Time) (int, []string) {
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return 0, []string{fmt.Sprintf("error reading %s sheet", sheetName)}
	}

	headers := rows[0]
	mapper := NewHeaderMapper(headers)

	var warnings []string
	inserted := 0

	// Header names (not values!)
	tsCol, hasTS := mapper.FindTimestampHeader()
	tankNoCol, _ := mapper.FindHeader("tank_no", "tank", "tank_id", "Tank ID")

	// Capacity column (may be liters or m3)
	capCol, _ := mapper.FindHeader("capacity", "Capacity(m3)", "volume", "volume_liters")

	// Current volume column (often "Current Level(m3)" in your sheet)
	curCol, _ := mapper.FindHeader("current", "Current Level(m3)", "current_level", "current_volume", "volume_liters")

	tempCol, _ := mapper.FindHeader("temp", "temperature", "temp_c")

	// for extra_json; keep the *source* headers that we read
	mappedCols := []string{}
	if tsCol != "" {
		mappedCols = append(mappedCols, tsCol)
	}
	if tankNoCol != "" {
		mappedCols = append(mappedCols, tankNoCol)
	}
	if capCol != "" {
		mappedCols = append(mappedCols, capCol)
	}
	if curCol != "" {
		mappedCols = append(mappedCols, curCol)
	}
	if tempCol != "" {
		mappedCols = append(mappedCols, tempCol)
	}

	// helper to detect m3 headers
	isM3Header := func(h string) bool {
		h = strings.ToLower(h)
		return strings.Contains(h, "(m3)") || strings.Contains(h, "m3")
	}

	for i := 1; i < len(rows); i++ {
		// map row by header -> cell
		row := make(map[string]string, len(headers))
		for j, cell := range rows[i] {
			if j < len(headers) {
				row[headers[j]] = cell
			}
		}

		// timestamp
		ts := defaultTS
		if hasTS && tsCol != "" {
			if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
				ts = parsedTS
			}
		}

		var numOnly = regexp.MustCompile(`\d+`)

		var tankNo *int
		if tankNoCol != "" {
			match := numOnly.FindString(row[tankNoCol]) // extract only digits
			if match != "" {
				if val, err := strconv.Atoi(match); err == nil {
					tankNo = &val // assign pointer to the parsed int
				}
			}
		}

		// capacity (liters)
		var capLiters *float64
		if capCol != "" {
			if v, _ := ParseFloat(row[capCol]); v != nil {
				val := *v
				if isM3Header(capCol) {
					val *= 1000.0
				}
				capLiters = &val
			}
		}

		// current volume (liters) — prefer explicit "current" column; fallback to capCol if that's actually the only volume column
		var curLiters *float64
		if curCol != "" {
			if v, _ := ParseFloat(row[curCol]); v != nil {
				val := *v
				if isM3Header(curCol) {
					val *= 1000.0
				}
				curLiters = &val
			}
		} else if capCol != "" {
			// Some sheets only provide one volume column; treat it as current volume
			if v, _ := ParseFloat(row[capCol]); v != nil {
				val := *v
				if isM3Header(capCol) {
					val *= 1000.0
				}
				curLiters = &val
			}
		}

		// temperature
		var tempC *float64
		if tempCol != "" {
			tempC, _ = ParseFloat(row[tempCol])
		}

		// level percent
		var levelPercent *float64
		if curLiters != nil && capLiters != nil && *capLiters > 0 {
			val := (*curLiters / *capLiters) * 100.0
			levelPercent = &val
		}

		// Validate using current volume (liters) and temp
		if warns := ValidateFuelData(levelPercent, curLiters, tempC); len(warns) > 0 {
			warnings = append(warnings, fmt.Sprintf("row %d fuel: %s", i+1, strings.Join(warns, ", ")))
			continue
		}

		// Build extra JSON from raw columns we used
		extraJSON, _ := BuildExtraJSON(row, mappedCols)

		// Hash
		hashKeys := []string{}
		if tankNo != nil {
			hashKeys = append(hashKeys, fmt.Sprintf("tank_no:%d", *tankNo))
		}
		hashKeys = append(hashKeys, string(extraJSON))
		rowHash := util.HashRow(vesselID, ts, "fuel", hashKeys...)

		// Insert (volume_liters = current volume in liters)
		_, err := p.db.Exec(`
			INSERT OR IGNORE INTO fuel_tank_readings 
			(vessel_id, tank_no, ts, level_percent, volume_liters, temp_c, row_hash, extra_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			vesselID,
			tankNo,
			ts,
			levelPercent,
			curLiters,
			tempC,
			rowHash,
			extraJSON,
		)
		if err == nil {
			inserted++
		} else {
			warnings = append(warnings, fmt.Sprintf("row %d fuel insert error: %v", i+1, err))
		}
	}

	return inserted, warnings
}

func (p *XLSXProcessor) processGeneratorSheet(f *excelize.File, sheetName string, vesselID int64, defaultTS time.Time) (int, []string) {
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return 0, []string{fmt.Sprintf("error reading %s sheet", sheetName)}
	}

	headers := rows[0]
	mapper := NewHeaderMapper(headers)

	var warnings []string
	inserted := 0

	tsCol, hasTS := mapper.FindTimestampHeader()
	genNoCol, _ := mapper.FindHeader("gen_no", "generator", "gen", "generator_no")
	loadCol, _ := mapper.FindHeader("load", "load_kw", "power")
	voltageCol, _ := mapper.FindHeader("voltage", "volt", "voltage_v")
	freqCol, _ := mapper.FindHeader("frequency", "freq", "frequency_hz")
	fuelRateCol, _ := mapper.FindHeader("fuel_rate", "fuel_rate_lph", "consumption")

	mappedCols := []string{tsCol, genNoCol, loadCol, voltageCol, freqCol, fuelRateCol}

	for i := 1; i < len(rows); i++ {
		row := make(map[string]string)
		for j, cell := range rows[i] {
			if j < len(headers) {
				row[headers[j]] = cell
			}
		}

		// Parse timestamp
		ts := defaultTS
		if hasTS && tsCol != "" {
			if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
				ts = parsedTS
			}
		}

		// Parse fields
		var genNo *int
		var loadKW, voltageV, frequencyHz, fuelRateLPH *float64

		var numOnly = regexp.MustCompile(`\d+`)

		if genNoCol != "" {
			match := numOnly.FindString(row[genNoCol]) // extract only digits
			if match != "" {
				if val, err := strconv.Atoi(match); err == nil {
					genNo = &val // assign pointer to the parsed int
				}
			}
		}
		if loadCol != "" {
			loadKW, _ = ParseFloat(row[loadCol])
		}
		if voltageCol != "" {
			voltageV, _ = ParseFloat(row[voltageCol])
		}
		if freqCol != "" {
			frequencyHz, _ = ParseFloat(row[freqCol])
		}
		if fuelRateCol != "" {
			fuelRateLPH, _ = ParseFloat(row[fuelRateCol])
		}

		// Validate
		if warns := ValidateGeneratorData(loadKW, voltageV, frequencyHz, fuelRateLPH); len(warns) > 0 {
			warnings = append(warnings, fmt.Sprintf("row %d generators: %s", i+1, strings.Join(warns, ", ")))
			continue
		}

		// Build extra JSON
		extraJSON, _ := BuildExtraJSON(row, mappedCols)

		// Create row hash
		hashKeys := []string{}
		if genNo != nil {
			hashKeys = append(hashKeys, fmt.Sprintf("gen_no:%d", *genNo))
		}
		hashKeys = append(hashKeys, string(extraJSON))
		rowHash := util.HashRow(vesselID, ts, "generators", hashKeys...)

		// Insert
		_, err := p.db.Exec(`
			INSERT OR IGNORE INTO generator_readings 
			(vessel_id, gen_no, ts, load_kw, voltage_v, frequency_hz, fuel_rate_lph, row_hash, extra_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			vesselID, genNo, ts, loadKW, voltageV, frequencyHz, fuelRateLPH, rowHash, extraJSON,
		)
		if err == nil {
			inserted++
		}
	}

	return inserted, warnings
}

func (p *XLSXProcessor) processCCTVSheet(f *excelize.File, sheetName string, vesselID int64, defaultTS time.Time) (int, []string) {
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return 0, []string{fmt.Sprintf("error reading %s sheet", sheetName)}
	}

	headers := rows[0]
	mapper := NewHeaderMapper(headers)

	var warnings []string
	inserted := 0

	tsCol, hasTS := mapper.FindTimestampHeader()
	camIDCol, _ := mapper.FindHeader("cam_id", "camera", "camera_id", "cam")
	statusCol, _ := mapper.FindHeader("status", "state")
	uptimeCol, _ := mapper.FindHeader("uptime", "uptime_percent", "availability")

	mappedCols := []string{tsCol, camIDCol, statusCol, uptimeCol}

	for i := 1; i < len(rows); i++ {
		row := make(map[string]string)
		for j, cell := range rows[i] {
			if j < len(headers) {
				row[headers[j]] = cell
			}
		}

		// Parse timestamp
		ts := defaultTS
		if hasTS && tsCol != "" {
			if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
				ts = parsedTS
			}
		}

		// Parse fields
		var camID, status *string
		var uptimePercent *float64

		if camIDCol != "" && row[camIDCol] != "" {
			val := row[camIDCol]
			camID = &val
		}
		if statusCol != "" && row[statusCol] != "" {
			val := row[statusCol]
			status = &val
		}
		if uptimeCol != "" {
			uptimePercent, _ = ParseFloat(row[uptimeCol])
		}

		// Build extra JSON
		extraJSON, _ := BuildExtraJSON(row, mappedCols)

		// Create row hash
		hashKeys := []string{}
		if camID != nil {
			hashKeys = append(hashKeys, fmt.Sprintf("cam_id:%s", *camID))
		}
		hashKeys = append(hashKeys, string(extraJSON))
		rowHash := util.HashRow(vesselID, ts, "cctv", hashKeys...)

		// Insert
		_, err := p.db.Exec(`
			INSERT OR IGNORE INTO cctv_status_readings 
			(vessel_id, cam_id, ts, status, uptime_percent, row_hash, extra_json)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			vesselID, camID, ts, status, uptimePercent, rowHash, extraJSON,
		)
		if err == nil {
			inserted++
		}
	}

	return inserted, warnings
}

func (p *XLSXProcessor) processImpactSheet(f *excelize.File, sheetName string, vesselID int64, defaultTS time.Time) (int, []string) {
	rows, err := f.GetRows(sheetName)
	if err != nil || len(rows) < 2 {
		return 0, []string{fmt.Sprintf("error reading %s sheet", sheetName)}
	}

	headers := rows[0]
	mapper := NewHeaderMapper(headers)

	var warnings []string
	inserted := 0

	tsCol, hasTS := mapper.FindTimestampHeader()
	sensorIDCol, _ := mapper.FindHeader("sensor_id", "sensor", "device_id")
	accelCol, _ := mapper.FindHeader("accel", "acceleration", "accel_g")
	shockCol, _ := mapper.FindHeader("shock", "shock_g", "impact")
	notesCol, _ := mapper.FindHeader("notes", "note", "comment")

	mappedCols := []string{tsCol, sensorIDCol, accelCol, shockCol, notesCol}

	for i := 1; i < len(rows); i++ {
		row := make(map[string]string)
		for j, cell := range rows[i] {
			if j < len(headers) {
				row[headers[j]] = cell
			}
		}

		// Parse timestamp
		ts := defaultTS
		if hasTS && tsCol != "" {
			if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
				ts = parsedTS
			}
		}

		// Parse fields
		var sensorID, notes *string
		var accelG, shockG *float64

		if sensorIDCol != "" && row[sensorIDCol] != "" {
			val := row[sensorIDCol]
			sensorID = &val
		}
		if accelCol != "" {
			accelG, _ = ParseFloat(row[accelCol])
		}
		if shockCol != "" {
			shockG, _ = ParseFloat(row[shockCol])
		}
		if notesCol != "" && row[notesCol] != "" {
			val := row[notesCol]
			notes = &val
		}

		// Build extra JSON
		extraJSON, _ := BuildExtraJSON(row, mappedCols)

		// Create row hash
		hashKeys := []string{}
		if sensorID != nil {
			hashKeys = append(hashKeys, fmt.Sprintf("sensor_id:%s", *sensorID))
		}
		hashKeys = append(hashKeys, string(extraJSON))
		rowHash := util.HashRow(vesselID, ts, "impact", hashKeys...)

		// Insert
		_, err := p.db.Exec(`
			INSERT OR IGNORE INTO impact_vibration_readings 
			(vessel_id, sensor_id, ts, accel_g, shock_g, notes, row_hash, extra_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			vesselID, sensorID, ts, accelG, shockG, notes, rowHash, extraJSON,
		)
		if err == nil {
			inserted++
		}
	}

	return inserted, warnings
}

func (p *XLSXProcessor) updateStreamLatest(vesselID int64, rowsInserted map[string]int, ts time.Time) {
	for stream, count := range rowsInserted {
		if count > 0 {
			_, _ = p.db.Exec(`
				INSERT OR REPLACE INTO vessel_stream_latest (vessel_id, stream, latest_ts)
				VALUES (?, ?, ?)`,
				vesselID, stream, ts,
			)
		}
	}
}
func (p *XLSXProcessor) processLocationFromShipInfo(headers, data []string, vesselID int64, defaultTS time.Time, mapper *HeaderMapper) (int, []string) {
	var warnings []string

	// Create row map
	row := make(map[string]string)
	for i, cell := range data {
		if i < len(headers) {
			row[headers[i]] = cell
		}
	}

	// Parse timestamp
	ts := defaultTS
	if tsCol, hasTS := mapper.FindTimestampHeader(); hasTS && tsCol != "" {
		if parsedTS, err := ParseTimestamp(row[tsCol]); err == nil {
			ts = parsedTS
		}
	}

	// Parse location fields
	var latitude, longitude, course, speed *float64
	var status *string

	if latCol, found := mapper.FindHeader("latitude", "lat"); found {
		latitude, _ = ParseFloat(row[latCol])
	}

	if lonCol, found := mapper.FindHeader("longitude", "lon", "lng"); found {
		longitude, _ = ParseFloat(row[lonCol])
	}

	if courseCol, found := mapper.FindHeader("course", "heading", "bearing"); found {
		course, _ = ParseFloat(row[courseCol])
	}

	if speedCol, found := mapper.FindHeader("speed", "speed_knots", "speed(knots)"); found {
		speed, _ = ParseFloat(row[speedCol])
	}

	if statusCol, found := mapper.FindHeader("status", "vessel_status", "nav_status"); found && row[statusCol] != "" {
		val := row[statusCol]
		status = &val
	}

	// Validate location data
	if warns := ValidateLocationData(latitude, longitude, course, speed); len(warns) > 0 {
		warnings = append(warnings, fmt.Sprintf("location data: %s", strings.Join(warns, ", ")))
		return 0, warnings
	}

	// Skip if no location data
	if latitude == nil && longitude == nil && course == nil && speed == nil && status == nil {
		return 0, warnings
	}

	// Build extra JSON for unmapped columns
	mappedCols := []string{}
	for _, h := range headers {
		headerLower := strings.ToLower(h)
		if strings.Contains(headerLower, "lat") ||
			strings.Contains(headerLower, "lon") ||
			strings.Contains(headerLower, "course") ||
			strings.Contains(headerLower, "speed") ||
			strings.Contains(headerLower, "status") ||
			strings.Contains(headerLower, "time") ||
			strings.Contains(headerLower, "name") ||
			strings.Contains(headerLower, "imo") {
			mappedCols = append(mappedCols, h)
		}
	}

	extraJSON, _ := BuildExtraJSON(row, mappedCols)

	// Create row hash
	hashKeys := []string{}
	if status != nil {
		hashKeys = append(hashKeys, fmt.Sprintf("status:%s", *status))
	}
	hashKeys = append(hashKeys, string(extraJSON))
	rowHash := util.HashRow(vesselID, ts, "location", hashKeys...)

	// Insert location reading
	_, err := p.db.Exec(`
		INSERT OR IGNORE INTO location_readings 
		(vessel_id, ts, latitude, longitude, course_degrees, speed_knots, status, row_hash, extra_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		vesselID, ts, latitude, longitude, course, speed, status, rowHash, extraJSON,
	)
	if err == nil {
		return 1, warnings
	}

	return 0, warnings
}
