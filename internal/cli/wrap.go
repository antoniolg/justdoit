package cli

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

func wrapText(text string, width int) string {
	if width <= 0 {
		return text
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := []string{}
	line := ""
	lineWidth := 0
	for _, word := range words {
		wordWidth := runewidth.StringWidth(word)
		if line == "" {
			line = word
			lineWidth = wordWidth
			continue
		}
		if lineWidth+1+wordWidth > width {
			lines = append(lines, line)
			line = word
			lineWidth = wordWidth
			continue
		}
		line += " " + word
		lineWidth += 1 + wordWidth
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
