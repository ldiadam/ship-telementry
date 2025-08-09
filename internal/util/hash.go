package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

func SHA256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func HashRow(vesselID int64, ts time.Time, stream string, keys ...string) string {
	// Sort keys for consistent hashing
	sort.Strings(keys)

	parts := []string{
		stream,
		fmt.Sprintf("%d", vesselID),
		ts.Format(time.RFC3339),
	}
	parts = append(parts, keys...)

	combined := strings.Join(parts, "|")
	return SHA256Hex([]byte(combined))
}
