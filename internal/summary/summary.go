package summary

import (
	"fmt"
	"net/url"
	"strings"
)

func LinkID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil {
		if id := strings.TrimSpace(u.Query().Get("id")); id != "" {
			return id
		}
	}
	return strings.TrimSpace(raw)
}

func Startup(name, channelID, bot string) string {
	return fmt.Sprintf("watch %s ch=%s bot=%q — Approve / Deny (Ctrl+C exits)", name, channelID, bot)
}

func LoadedPending(n int) string {
	return fmt.Sprintf("loaded %d pending cart(s)", n)
}

func CartPending(n int, link string) string {
	return fmt.Sprintf("#%d PENDING id=%s", n, LinkID(link))
}

func Approved(n int, who, link string) string {
	return fmt.Sprintf("#%d OK id=%s by=%s", n, LinkID(link), who)
}

func Denied(who, link string) string {
	return fmt.Sprintf("DENY id=%s by=%s", LinkID(link), who)
}
