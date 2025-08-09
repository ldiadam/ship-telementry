package api

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func EncodeCursor(ts time.Time, id int64) string {
	cursor := fmt.Sprintf("%s|%d", ts.Format(time.RFC3339), id)
	return base64.StdEncoding.EncodeToString([]byte(cursor))
}

func DecodeCursor(s string) (time.Time, int64, error) {
	if s == "" {
		return time.Time{}, 0, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}

	parts := strings.Split(string(decoded), "|")
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}

	ts, err := time.Parse(time.RFC3339, parts[0])
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid timestamp in cursor")
	}

	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid id in cursor")
	}

	return ts, id, nil
}
