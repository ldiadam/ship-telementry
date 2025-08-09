package ingest

import (
	"testing"
)

func TestHeaderMapper(t *testing.T) {
	headers := []string{"Engine RPM", "Temperature C", "Oil Pressure", "Timestamp"}
	mapper := NewHeaderMapper(headers)

	// Test exact match
	if header, found := mapper.FindHeader("engine_rpm"); !found || header != "Engine RPM" {
		t.Errorf("Expected to find 'Engine RPM', got %s, found: %v", header, found)
	}

	// Test substring match
	if header, found := mapper.FindHeader("rpm"); !found || header != "Engine RPM" {
		t.Errorf("Expected to find 'Engine RPM' via substring, got %s, found: %v", header, found)
	}

	// Test timestamp detection
	if header, found := mapper.FindTimestampHeader(); !found || header != "Timestamp" {
		t.Errorf("Expected to find 'Timestamp', got %s, found: %v", header, found)
	}
}

func TestParseFloat(t *testing.T) {
	// Valid float
	if val, err := ParseFloat("123.45"); err != nil || val == nil || *val != 123.45 {
		t.Errorf("Expected 123.45, got %v, err: %v", val, err)
	}

	// Empty string
	if val, err := ParseFloat(""); err != nil || val != nil {
		t.Errorf("Expected nil for empty string, got %v, err: %v", val, err)
	}

	// Invalid float
	if val, err := ParseFloat("invalid"); err == nil {
		t.Errorf("Expected error for invalid float, got %v", val)
	}
}

func TestParseTimestamp(t *testing.T) {
	// Valid ISO 8601
	if ts, err := ParseTimestamp("2025-08-08T10:00:00Z"); err != nil {
		t.Errorf("Expected valid timestamp, got error: %v", err)
	} else if ts.Year() != 2025 {
		t.Errorf("Expected year 2025, got %d", ts.Year())
	}

	// Valid date only
	if ts, err := ParseTimestamp("2025-08-08"); err != nil {
		t.Errorf("Expected valid date, got error: %v", err)
	} else if ts.Year() != 2025 {
		t.Errorf("Expected year 2025, got %d", ts.Year())
	}

	// Invalid timestamp
	if _, err := ParseTimestamp("invalid"); err == nil {
		t.Errorf("Expected error for invalid timestamp")
	}
}

func TestValidateEngineData(t *testing.T) {
	// Valid data
	rpm := 1500.0
	temp := 80.0
	pressure := 5.0

	warnings := ValidateEngineData(&rpm, &temp, &pressure)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for valid data, got: %v", warnings)
	}

	// Invalid RPM
	negativeRPM := -100.0
	warnings = ValidateEngineData(&negativeRPM, &temp, &pressure)
	if len(warnings) == 0 {
		t.Errorf("Expected warning for negative RPM")
	}
}

func TestValidateFuelData(t *testing.T) {
	// Valid data
	level := 75.0
	volume := 1000.0
	temp := 25.0

	warnings := ValidateFuelData(&level, &volume, &temp)
	if len(warnings) != 0 {
		t.Errorf("Expected no warnings for valid data, got: %v", warnings)
	}

	// Invalid level
	invalidLevel := 150.0
	warnings = ValidateFuelData(&invalidLevel, &volume, &temp)
	if len(warnings) == 0 {
		t.Errorf("Expected warning for invalid fuel level")
	}
}
