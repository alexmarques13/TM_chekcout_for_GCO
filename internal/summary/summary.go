package summary

import (
	"encoding/json"
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

func Dash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func ShortError(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "-"
	}

	var items []map[string]any
	if err := json.Unmarshal([]byte(raw), &items); err == nil && len(items) > 0 {
		return formatErrorItem(items[0])
	}

	var item map[string]any
	if err := json.Unmarshal([]byte(raw), &item); err == nil && len(item) > 0 {
		return formatErrorItem(item)
	}

	return oneLine(raw, 120)
}

func formatErrorItem(item map[string]any) string {
	code := strings.TrimSpace(fmt.Sprint(item["code"]))
	msg := strings.TrimSpace(fmt.Sprint(item["message"]))
	if msg == "" || msg == "<nil>" {
		msg = strings.TrimSpace(fmt.Sprint(item["error"]))
	}
	if code != "" && code != "<nil>" && msg != "" && msg != "<nil>" {
		return code + ": " + msg
	}
	if msg != "" && msg != "<nil>" {
		return msg
	}
	if code != "" && code != "<nil>" {
		return code
	}
	return oneLine(fmt.Sprint(item), 120)
}

func oneLine(s string, max int) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "-"
	}
	if max > 0 && len(s) > max {
		return s[:max] + "..."
	}
	return s
}

func Totals(done, ok, fail int, spent float64) string {
	return fmt.Sprintf("done=%d ok=%d fail=%d spent=$%.2f", done, ok, fail, spent)
}

func Startup(name, channelID, bot string) string {
	return fmt.Sprintf("watch %s ch=%s bot=%q — Approve to checkout (Ctrl+C exits)", name, channelID, bot)
}

func LoadedPending(n int) string {
	return fmt.Sprintf("loaded %d pending cart(s)", n)
}

func CartPending(n int, link string) string {
	return fmt.Sprintf("#%d PENDING id=%s", n, LinkID(link))
}

func Approved(n int, who, link string, cardIdx, cardTotal int, last4 string) string {
	return fmt.Sprintf("#%d RUN id=%s by=%s card=%d/%d ****%s", n, LinkID(link), who, cardIdx+1, cardTotal, last4)
}

func Denied(who, link string) string {
	return fmt.Sprintf("DENY id=%s by=%s", LinkID(link), who)
}

type Result struct {
	Index     int
	Link      string
	Success   bool
	Expired   bool
	EventID   string
	OrderID   string
	CardLast4 string
	SpentUSD  float64
	Error     string
	Done      int
	OK        int
	Fail      int
	TotalSpent float64
}

func CheckoutLine(r Result) string {
	id := LinkID(r.Link)
	totals := Totals(r.Done, r.OK, r.Fail, r.TotalSpent)
	card := Dash(r.CardLast4)
	if card != "-" {
		card = "****" + card
	}

	if r.Success {
		return fmt.Sprintf("#%d OK   id=%s event=%s order=%s card=%s spent=$%.2f | %s",
			r.Index, id, Dash(r.EventID), Dash(r.OrderID), card, r.SpentUSD, totals)
	}

	status := "FAIL"
	if r.Expired {
		status = "EXPIRED"
	}
	return fmt.Sprintf("#%d %s id=%s event=%s card=%s spent=$%.2f err=%s | %s",
		r.Index, status, id, Dash(r.EventID), card, r.SpentUSD, ShortError(r.Error), totals)
}
