package api

import (
	"testing"
	"time"
)

func TestEncodeDecode(t *testing.T) {
	ts := time.Date(2025, 8, 8, 10, 0, 0, 0, time.UTC)
	id := int64(123)

	// Encode
	cursor := EncodeCursor(ts, id)
	if cursor == "" {
		t.Errorf("Expected non-empty cursor")
	}

	// Decode
	decodedTS, decodedID, err := DecodeCursor(cursor)
	if err != nil {
		t.Errorf("Expected no error decoding, got: %v", err)
	}

	if !decodedTS.Equal(ts) {
		t.Errorf("Expected timestamp %v, got %v", ts, decodedTS)
	}

	if decodedID != id {
		t.Errorf("Expected ID %d, got %d", id, decodedID)
	}
}

func TestDecodeEmpty(t *testing.T) {
	ts, id, err := DecodeCursor("")
	if err != nil {
		t.Errorf("Expected no error for empty cursor, got: %v", err)
	}

	if !ts.IsZero() {
		t.Errorf("Expected zero timestamp for empty cursor")
	}

	if id != 0 {
		t.Errorf("Expected zero ID for empty cursor")
	}
}

func TestDecodeInvalid(t *testing.T) {
	_, _, err := DecodeCursor("invalid")
	if err == nil {
		t.Errorf("Expected error for invalid cursor")
	}
}
