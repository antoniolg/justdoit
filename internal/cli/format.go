package cli

import (
	"os"

	"github.com/mattn/go-isatty"
)

const (
	colorReset = "\033[0m"
	colorGray  = "\033[90m"
)

func gray(text string) string {
	if !useColor() {
		return text
	}
	return colorGray + text + colorReset
}

func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}
