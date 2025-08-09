package ingest

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// HeaderMapper provides fuzzy matching for column headers
type HeaderMapper struct {
	headers map[string]string // normalized -> original
}

func NewHeaderMapper(headers []string) *HeaderMapper {
	hm := &HeaderMapper{
		headers: make(map[string]string),
	}

	for _, h := range headers {
		normalized := normalizeHeader(h)
		hm.headers[normalized] = h
	}

	return hm
}

func normalizeHeader(header string) string {
	h := strings.TrimSpace(strings.ToLower(header))
	h = strings.ReplaceAll(h, " ", "_")
	h = strings.ReplaceAll(h, "-", "_")
	return h
}

func (hm *HeaderMapper) FindHeader(patterns ...string) (string, bool) {
	for _, pattern := range patterns {
		// Exact match first
		if original, exists := hm.headers[pattern]; exists {
			return original, true
		}

		// Substring match
		for normalized, original := range hm.headers {
			if strings.Contains(normalized, pattern) {
				return original, true
			}
		}
	}
	return "", false
}

func (hm *HeaderMapper) FindTimestampHeader() (string, bool) {
	return hm.FindHeader(
		"timestamp", "ts", "time", "date", "datetime",
		"date_time", "time_stamp", "record_time", "log_time",
		"created_at", "recorded_at", "sample_time", "measurement_time",
		"utc", "local_time", "system_time", "event_time",
	)
}

// ParseFloat safely parses a string to float64
func ParseFloat(s string) (*float64, error) {
	if s == "" {
		return nil, nil
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil, err
	}

	return &val, nil
}

// ParseInt safely parses a string to int
func ParseInt(s string) (*int, error) {
	if s == "" {
		return nil, nil
	}

	val, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}

	return &val, nil
}

// ParseTimestamp attempts to parse various timestamp formats
func ParseTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04",
		"2006-01-02",
		"15:04:05",
		"15:04",
	}

	s = strings.TrimSpace(s)

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}

// ValidateEngineData validates engine reading data
func ValidateEngineData(rpm, temp, pressure *float64) []string {
	var warnings []string

	if rpm != nil && *rpm < 0 {
		warnings = append(warnings, "negative rpm")
	}

	if pressure != nil && *pressure < 0 {
		warnings = append(warnings, "negative oil pressure")
	}

	return warnings
}

// ValidateFuelData validates fuel tank reading data
func ValidateFuelData(level, volume, temp *float64) []string {
	var warnings []string

	if level != nil && (*level < 0 || *level > 100) {
		warnings = append(warnings, "invalid fuel level percentage")
	}

	if volume != nil && *volume < 0 {
		warnings = append(warnings, "negative fuel volume")
	}

	return warnings
}

// ValidateGeneratorData validates generator reading data
func ValidateGeneratorData(load, voltage, frequency, fuelRate *float64) []string {
	var warnings []string

	if load != nil && *load < 0 {
		warnings = append(warnings, "negative generator load")
	}

	if voltage != nil && *voltage < 0 {
		warnings = append(warnings, "negative voltage")
	}

	if frequency != nil && (*frequency < 45 || *frequency > 70) {
		warnings = append(warnings, "frequency out of range (45-70 Hz)")
	}

	if fuelRate != nil && *fuelRate < 0 {
		warnings = append(warnings, "negative fuel rate")
	}

	return warnings
}

// BuildExtraJSON creates JSON from unmapped columns
func BuildExtraJSON(row map[string]string, mappedCols []string) (json.RawMessage, error) {
	extra := make(map[string]string)

	for col, val := range row {
		mapped := false
		for _, mappedCol := range mappedCols {
			if col == mappedCol {
				mapped = true
				break
			}
		}

		if !mapped && val != "" {
			extra[col] = val
		}
	}

	if len(extra) == 0 {
		return json.RawMessage("{}"), nil
	}

	data, err := json.Marshal(extra)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(data), nil
}

// ValidateLocationData validates location reading data
func ValidateLocationData(latitude, longitude, course, speed *float64) []string {
	var warnings []string

	if latitude != nil && (*latitude < -90 || *latitude > 90) {
		warnings = append(warnings, "latitude out of range (-90 to 90)")
	}

	if longitude != nil && (*longitude < -180 || *longitude > 180) {
		warnings = append(warnings, "longitude out of range (-180 to 180)")
	}

	if course != nil && (*course < 0 || *course > 360) {
		warnings = append(warnings, "course out of range (0-360 degrees)")
	}

	if speed != nil && *speed < 0 {
		warnings = append(warnings, "negative speed")
	}

	return warnings
}
