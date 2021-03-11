package gitlab

import (
	"time"
)

// ParseDateTime validates that dateTime complies with the ISO 8601 format
// time.RFC3339 = "2006-01-02T15:04:05Z07:00" which is subset of the ISO 8601 which allows using timezones
func ParseDateTime(dateTime string) (time.Time, error) {
	return time.Parse(time.RFC3339, dateTime)
}
