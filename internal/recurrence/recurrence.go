package recurrence

import (
	"fmt"
	"strings"
)

var dayMap = map[string]string{
	"monday":   "MO",
	"mon":      "MO",
	"lunes":    "MO",
	"lun":      "MO",
	"tuesday":  "TU",
	"tue":      "TU",
	"martes":   "TU",
	"mar":      "TU",
	"wednesday":"WE",
	"wed":      "WE",
	"miercoles":"WE",
	"miércoles":"WE",
	"mie":      "WE",
	"jueves":   "TH",
	"thu":      "TH",
	"thursday": "TH",
	"viernes":  "FR",
	"fri":      "FR",
	"friday":   "FR",
	"sabado":   "SA",
	"sábado":   "SA",
	"sat":      "SA",
	"saturday": "SA",
	"domingo":  "SU",
	"sun":      "SU",
	"sunday":   "SU",
}

var dayOrder = []string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"}

func ParseEvery(input string) ([]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	if strings.HasPrefix(strings.ToUpper(input), "RRULE:") {
		return []string{input}, nil
	}
	rule, ok := parseRecurrence(input, true)
	if !ok {
		return nil, fmt.Errorf("unsupported recurrence: %s", input)
	}
	return []string{rule}, nil
}

func ExtractFromText(text string) (string, []string, bool) {
	input := strings.TrimSpace(text)
	if input == "" {
		return text, nil, false
	}
	rule, ok := parseRecurrence(input, false)
	if !ok {
		return text, nil, false
	}
	clean := stripRecurrenceTokens(input)
	return clean, []string{rule}, true
}

func parseRecurrence(input string, allowBareDays bool) (string, bool) {
	clean := normalize(input)
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(input)), "RRULE:") {
		return strings.TrimSpace(input), true
	}
	if allowBareDays || containsRecurrencePrefix(clean) {
		if days := parseDayCodes(clean); len(days) > 0 {
			return "RRULE:FREQ=WEEKLY;BYDAY=" + strings.Join(days, ","), true
		}
	}
	if containsAny(clean, []string{"daily", "every day", "cada dia", "cada día", "diario", "diaria"}) {
		return "RRULE:FREQ=DAILY", true
	}
	if containsAny(clean, []string{"weekly", "every week", "cada semana", "semanal"}) {
		return "RRULE:FREQ=WEEKLY", true
	}
	if containsAny(clean, []string{"monthly", "every month", "cada mes", "mensual"}) {
		return "RRULE:FREQ=MONTHLY", true
	}
	if containsAny(clean, []string{"yearly", "every year", "cada ano", "cada año", "anual"}) {
		return "RRULE:FREQ=YEARLY", true
	}
	if containsAny(clean, []string{"weekday", "weekdays", "laborable", "laborables"}) {
		return "RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", true
	}
	return "", false
}

func parseDayCodes(clean string) []string {
	found := map[string]bool{}
	for _, token := range strings.Fields(clean) {
		key := strings.Trim(token, " ,.;:")
		if code, ok := dayMap[key]; ok {
			found[code] = true
		}
	}
	result := []string{}
	for _, code := range dayOrder {
		if found[code] {
			result = append(result, code)
		}
	}
	return result
}

func containsRecurrencePrefix(text string) bool {
	return containsAny(text, []string{"cada", "every", "todos", "todas", "los", "las"})
}

func containsAny(text string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(text, n) {
			return true
		}
	}
	return false
}

func stripRecurrenceTokens(text string) string {
	replace := func(s, target string) string {
		return replaceOnceCaseInsensitive(s, target, "")
	}
	clean := text
	for _, token := range []string{"cada", "every", "todos", "todas", "los", "las", "daily", "weekly", "monthly", "yearly", "diario", "diaria", "semanal", "mensual", "anual", "weekday", "weekdays", "laborable", "laborables"} {
		clean = replace(clean, token)
	}
	for token := range dayMap {
		clean = replace(clean, token)
	}
	clean = replace(clean, "y")
	clean = replace(clean, "and")
	return normalizeSpaces(clean)
}

func normalize(input string) string {
	value := strings.ToLower(strings.TrimSpace(input))
	replacer := strings.NewReplacer(
		"á", "a",
		"é", "e",
		"í", "i",
		"ó", "o",
		"ú", "u",
		"ü", "u",
	)
	value = replacer.Replace(value)
	return value
}

func replaceOnceCaseInsensitive(text, target, repl string) string {
	index := strings.Index(strings.ToLower(text), strings.ToLower(target))
	if index == -1 {
		return text
	}
	return text[:index] + repl + text[index+len(target):]
}

func normalizeSpaces(text string) string {
	fields := strings.Fields(text)
	return strings.TrimSpace(strings.Join(fields, " "))
}
