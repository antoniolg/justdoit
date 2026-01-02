package recurrence

import (
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
)

func NextOccurrence(rule string, start time.Time, after time.Time, loc *time.Location) (time.Time, bool, error) {
	clean := strings.TrimSpace(rule)
	if clean == "" {
		return time.Time{}, false, nil
	}
	location := loc
	if location == nil {
		location = time.Local
	}
	option, err := rrule.StrToROptionInLocation(clean, location)
	if err != nil {
		return time.Time{}, false, err
	}
	if !start.IsZero() {
		option.Dtstart = start.In(location)
	} else if option.Dtstart.IsZero() {
		option.Dtstart = after.In(location)
	}
	parsed, err := rrule.NewRRule(*option)
	if err != nil {
		return time.Time{}, false, err
	}
	next := parsed.After(after.In(location), false)
	if next.IsZero() {
		return time.Time{}, false, nil
	}
	return next.In(location), true, nil
}
