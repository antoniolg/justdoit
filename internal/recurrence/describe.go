package recurrence

import (
	"fmt"
	"sort"
	"strings"
	"time"

	rrule "github.com/teambition/rrule-go"
)

func Describe(rule string, loc *time.Location) (string, bool) {
	clean := strings.TrimSpace(rule)
	if clean == "" {
		return "", false
	}
	location := loc
	if location == nil {
		location = time.Local
	}
	option, err := rrule.StrToROptionInLocation(clean, location)
	if err != nil {
		return "", false
	}
	interval := option.Interval
	if interval <= 0 {
		interval = 1
	}
	switch option.Freq {
	case rrule.DAILY:
		if interval == 1 {
			return "cada dia", true
		}
		return fmt.Sprintf("cada %d dias", interval), true
	case rrule.WEEKLY:
		base := "cada semana"
		if interval > 1 {
			base = fmt.Sprintf("cada %d semanas", interval)
		}
		days := describeWeekdays(option.Byweekday)
		if days != "" {
			return fmt.Sprintf("%s (%s)", base, days), true
		}
		return base, true
	case rrule.MONTHLY:
		base := "cada mes"
		if interval > 1 {
			base = fmt.Sprintf("cada %d meses", interval)
		}
		if len(option.Bymonthday) > 0 {
			days := append([]int{}, option.Bymonthday...)
			sort.Ints(days)
			return fmt.Sprintf("%s el dia %s", base, joinInts(days)), true
		}
		days := describeWeekdays(option.Byweekday)
		if days != "" {
			return fmt.Sprintf("%s (%s)", base, days), true
		}
		return base, true
	case rrule.YEARLY:
		base := "cada ano"
		if interval > 1 {
			base = fmt.Sprintf("cada %d anos", interval)
		}
		return base, true
	default:
		return "", false
	}
}

func describeWeekdays(days []rrule.Weekday) string {
	if len(days) == 0 {
		return ""
	}
	labels := make([]string, 0, len(days))
	for _, day := range days {
		labels = append(labels, weekdayLabel(day.String()))
	}
	return strings.Join(labels, ", ")
}

func weekdayLabel(code string) string {
	switch strings.ToUpper(code) {
	case "MO":
		return "lun"
	case "TU":
		return "mar"
	case "WE":
		return "mie"
	case "TH":
		return "jue"
	case "FR":
		return "vie"
	case "SA":
		return "sab"
	case "SU":
		return "dom"
	default:
		return strings.ToLower(code)
	}
}

func joinInts(values []int) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%d", v))
	}
	return strings.Join(parts, ", ")
}
