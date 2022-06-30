package common

import (
	"strconv"
	"strings"
	"time"
)

// TimeRFC3339 is used specifically to marshal and unmarshal time to/from RFC3339 strings
// That's because the metadata is user-facing and using Go's built-in time parsing will not be portable
type TimeRFC3339 struct {
	time.Time
}

func NewTimeRFC3339(t time.Time) TimeRFC3339 {
	return TimeRFC3339{t}
}

func (t *TimeRFC3339) UnmarshalJSON(b []byte) error {
	var err error
	t.Time, err = time.Parse(time.RFC3339, strings.Trim(string(b), `"`))
	return err
}

func (t TimeRFC3339) MarshalJSON() ([]byte, error) {
	if t.IsZero() {
		return nil, nil
	}

	return []byte(strconv.Quote(t.Time.Format(time.RFC3339))), nil
}
