package color

import (
	"os"
)

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Red     = "\033[91m"
	Green   = "\033[92m"
	Yellow  = "\033[93m"
	Cyan    = "\033[96m"
)

var enabled bool

func Init() {
	if os.Getenv("NO_COLOR") != "" {
		return
	}
	fi, err := os.Stdout.Stat()
	if err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return
	}
	enableWindowsColor()
	enabled = true
}

func Paint(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return code + s + Reset
}

func BoldPaint(code, s string) string {
	if !enabled || s == "" {
		return s
	}
	return Bold + code + s + Reset
}
