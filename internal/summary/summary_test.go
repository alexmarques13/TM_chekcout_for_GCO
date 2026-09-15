package summary

import "testing"

func TestLinkID(t *testing.T) {
	got := LinkID("https://encsoft.app/api/bot/getLink?id=ABC123")
	if got != "ABC123" {
		t.Fatalf("got %q", got)
	}
}

func TestApproved(t *testing.T) {
	got := Approved(1, "alice", "https://encsoft.app/api/bot/getLink?id=ABC123")
	want := "#1 OK id=ABC123 by=alice"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
