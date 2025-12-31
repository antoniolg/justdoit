package agenda

import (
	"fmt"
	"sort"
	"time"

	"google.golang.org/api/calendar/v3"

	"justdoit/internal/timeparse"
)

type Slot struct {
	Start time.Time
	End   time.Time
}

func DayBounds(day time.Time, startClock, endClock string, loc *time.Location) (time.Time, time.Time, error) {
	start, err := timeparse.ParseClock(startClock, day, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end, err := timeparse.ParseClock(endClock, day, loc)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if !end.After(start) {
		return time.Time{}, time.Time{}, fmt.Errorf("workday_end must be after workday_start")
	}
	return start, end, nil
}

func FreeSlots(events []*calendar.Event, dayStart, dayEnd time.Time) []Slot {
	busy := normalizeEvents(events, dayStart.Location())
	if len(busy) == 0 {
		return []Slot{{Start: dayStart, End: dayEnd}}
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].Start.Before(busy[j].Start) })
	merged := []Slot{busy[0]}
	for _, b := range busy[1:] {
		last := &merged[len(merged)-1]
		if b.Start.After(last.End) {
			merged = append(merged, b)
			continue
		}
		if b.End.After(last.End) {
			last.End = b.End
		}
	}

	var free []Slot
	cursor := dayStart
	for _, b := range merged {
		if b.End.Before(dayStart) || b.Start.After(dayEnd) {
			continue
		}
		start := maxTime(cursor, dayStart)
		end := minTime(b.Start, dayEnd)
		if end.After(start) {
			free = append(free, Slot{Start: start, End: end})
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	if dayEnd.After(cursor) {
		free = append(free, Slot{Start: cursor, End: dayEnd})
	}
	return free
}

func normalizeEvents(events []*calendar.Event, loc *time.Location) []Slot {
	var slots []Slot
	for _, e := range events {
		start, end := eventTimes(e, loc)
		if start.IsZero() || end.IsZero() {
			continue
		}
		slots = append(slots, Slot{Start: start, End: end})
	}
	return slots
}

func eventTimes(e *calendar.Event, loc *time.Location) (time.Time, time.Time) {
	if e.Start == nil || e.End == nil {
		return time.Time{}, time.Time{}
	}
	if e.Start.DateTime != "" && e.End.DateTime != "" {
		start, err := time.Parse(time.RFC3339, e.Start.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		end, err := time.Parse(time.RFC3339, e.End.DateTime)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		return start.In(loc), end.In(loc)
	}
	if e.Start.Date != "" && e.End.Date != "" {
		start, err := time.ParseInLocation("2006-01-02", e.Start.Date, loc)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		end, err := time.ParseInLocation("2006-01-02", e.End.Date, loc)
		if err != nil {
			return time.Time{}, time.Time{}
		}
		return start, end
	}
	return time.Time{}, time.Time{}
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
