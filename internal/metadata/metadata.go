package metadata

import (
	"fmt"
	"strings"
)

func Append(text, key, value string) string {
	marker := fmt.Sprintf("%s=%s", key, value)
	if strings.Contains(text, marker) {
		return text
	}
	if text == "" {
		return marker
	}
	return strings.TrimSpace(text) + "\n" + marker
}

func Extract(text, key string) (string, bool) {
	prefix := key + "="
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), true
		}
	}
	return "", false
}
