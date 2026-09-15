package notify

import "testing"

func TestApprovedEmbedFields(t *testing.T) {
	embed := approvedEmbed(CartInfo{
		Who:  "alice",
		Link: "https://encsoft.app/api/bot/getLink?id=ABC123",
	})
	if embed["title"] != "Cart approved" {
		t.Fatalf("title: %v", embed["title"])
	}
	if embed["color"] != colorSuccess {
		t.Fatalf("color: %v", embed["color"])
	}
	fields := embed["fields"].([]map[string]any)
	want := []string{"By", "Link"}
	if len(fields) != len(want) {
		t.Fatalf("fields=%d want %d", len(fields), len(want))
	}
	for i, name := range want {
		if fields[i]["name"] != name || fields[i]["inline"] != true {
			t.Fatalf("field %d: %+v", i, fields[i])
		}
	}
}

func TestDeniedEmbedStyle(t *testing.T) {
	embed := deniedEmbed(CartInfo{
		Who:  "bob",
		Link: "https://encsoft.app/api/bot/getLink?id=DEF456",
	})
	if embed["title"] != "Cart denied" {
		t.Fatalf("title: %v", embed["title"])
	}
	if embed["color"] != colorFail {
		t.Fatalf("color: %v", embed["color"])
	}
	fields := embed["fields"].([]map[string]any)
	if len(fields) != 2 {
		t.Fatalf("deny fields=%d want 2", len(fields))
	}
}
