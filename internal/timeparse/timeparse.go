package timeparse

import (
	"fmt"
	"strings"
	"time"

	"github.com/tj/go-naturaldate"
)

func LoadLocation(name string) (*time.Location, error) {
	if name == "" || strings.EqualFold(name, "local") {
		return time.Local, nil
	}
	return time.LoadLocation(name)
}

func ParseDate(dateStr string, now time.Time, loc *time.Location) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	parsed, err := naturaldate.Parse(dateStr, now.In(loc))
	if err != nil {
		if t, parseErr := time.ParseInLocation("2006-01-02", dateStr, loc); parseErr == nil {
			return t, nil
		}
		return time.Time{}, err
	}
	return time.Date(parsed.Year(), parsed.Month(), parsed.Day(), 0, 0, 0, 0, loc), nil
}

func ParseClock(clock string, base time.Time, loc *time.Location) (time.Time, error) {
	parts := strings.Split(clock, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid clock: %s", clock)
	}
	hour, err := parseInt(parts[0])
	if err != nil {
		return time.Time{}, err
	}
	min, err := parseInt(parts[1])
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, loc), nil
}

func ParseTimeRange(timeStr string, baseDate, now time.Time, loc *time.Location) (time.Time, time.Time, error) {
	if timeStr == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("time is required")
	}
	if dur, err := time.ParseDuration(timeStr); err == nil {
		start := now.In(loc)
		end := start.Add(dur)
		return start, end, nil
	}
	parts := strings.Split(timeStr, "-")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid time range: %s", timeStr)
	}
	ref := now.In(loc)
	if !baseDate.IsZero() {
		ref = baseDate.In(loc)
	}
	start, err := naturaldate.Parse(strings.TrimSpace(parts[0]), ref.In(loc))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := naturaldate.Parse(strings.TrimSpace(parts[1]), ref.In(loc))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !baseDate.IsZero() {
		start = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), start.Hour(), start.Minute(), 0, 0, loc)
		end = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), end.Hour(), end.Minute(), 0, 0, loc)
	}
	if !end.After(start) {
		end = end.Add(24 * time.Hour)
	}
	return start, end, nil
}

func parseInt(value string) (int, error) {
	var i int
	_, err := fmt.Sscanf(value, "%d", &i)
	return i, err
}
