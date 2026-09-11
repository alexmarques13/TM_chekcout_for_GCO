package summary

import (
	"strings"
	"testing"
)

func TestShortErrorJSON(t *testing.T) {
	raw := `[{"code":"U201","message":"You have exceeded the maximum number of purchase attempts"}]`
	got := ShortError(raw)
	want := "U201: You have exceeded the maximum number of purchase attempts"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestShortErrorPlain(t *testing.T) {
	got := ShortError("checkout expired")
	if got != "checkout expired" {
		t.Fatalf("got %q", got)
	}
}

func TestCheckoutLineFail(t *testing.T) {
	line := CheckoutLine(Result{
		Index:      1,
		Link:       "https://encsoft.app/api/bot/getLink?id=ABC123",
		EventID:    "0F00635BEEFA7350",
		CardLast4:  "1111",
		Error:      `[{"code":"U201","message":"You have exceeded the maximum number of purchase attempts"}]`,
		Done:       1,
		Fail:       1,
	})
	if !strings.Contains(line, "id=ABC123") || !strings.Contains(line, "U201:") || !strings.Contains(line, "done=1") {
		t.Fatalf("unexpected line: %s", line)
	}
}
