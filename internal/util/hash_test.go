package util

import (
	"testing"
	"time"
)

func TestSHA256Hex(t *testing.T) {
	data := []byte("test data")
	hash := SHA256Hex(data)

	if len(hash) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash))
	}

	// Same data should produce same hash
	hash2 := SHA256Hex(data)
	if hash != hash2 {
		t.Errorf("Same data should produce same hash")
	}

	// Different data should produce different hash
	hash3 := SHA256Hex([]byte("different data"))
	if hash == hash3 {
		t.Errorf("Different data should produce different hash")
	}
}

func TestHashRow(t *testing.T) {
	vesselID := int64(123)
	ts := time.Date(2025, 8, 8, 10, 0, 0, 0, time.UTC)
	stream := "engines"
	keys := []string{"engine_no:1", "rpm:1500"}

	hash1 := HashRow(vesselID, ts, stream, keys...)
	hash2 := HashRow(vesselID, ts, stream, keys...)

	if hash1 != hash2 {
		t.Errorf("Same parameters should produce same hash")
	}

	// Different vessel ID should produce different hash
	hash3 := HashRow(456, ts, stream, keys...)
	if hash1 == hash3 {
		t.Errorf("Different vessel ID should produce different hash")
	}

	// Different timestamp should produce different hash
	ts2 := ts.Add(time.Minute)
	hash4 := HashRow(vesselID, ts2, stream, keys...)
	if hash1 == hash4 {
		t.Errorf("Different timestamp should produce different hash")
	}
}
