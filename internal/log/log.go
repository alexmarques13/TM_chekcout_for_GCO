package log

import (
	"fmt"
	"strings"
	"time"
)

var Quiet = true

func Request(method, url string) {
	if Quiet {
		return
	}
	fmt.Printf("[%s] → %s %s\n", ts(), strings.ToUpper(method), url)
}

func Response(method, url string, status int, note string) {
	if Quiet {
		return
	}
	msg := fmt.Sprintf("← %d", status)
	if note != "" {
		msg += " " + note
	}
	fmt.Printf("[%s] %s %s %s\n", ts(), strings.ToUpper(method), url, msg)
}

func Action(action string) {
	if Quiet {
		return
	}
	fmt.Printf("[%s] • %s\n", ts(), action)
}

func OK(msg string) {
	if Quiet {
		return
	}
	fmt.Printf("[%s] ✓ %s\n", ts(), msg)
}

func Fail(msg string) {
	if Quiet {
		return
	}
	fmt.Printf("[%s] ✗ %s\n", ts(), msg)
}

func ts() string {
	return time.Now().Format("15:04:05.000")
}
