package notify

import "testing"

func TestSuccessEmbedFields(t *testing.T) {
	embed := successEmbed(CheckoutInfo{
		Store:     "US",
		EventID:   "0D006378E4049EE0",
		Account:   "acct@example.com",
		Profile:   "profile@example.com",
		CardLast4: "4242",
		OrderID:   "53-37593/FLO",
	})
	if embed["title"] != "Successful checkout" {
		t.Fatalf("title: %v", embed["title"])
	}
	if embed["color"] != colorSuccess {
		t.Fatalf("color: %v", embed["color"])
	}
	fields := embed["fields"].([]map[string]any)
	want := []string{"Store", "Event ID", "Account", "Profile", "Card", "Order"}
	if len(fields) != len(want) {
		t.Fatalf("fields=%d want %d", len(fields), len(want))
	}
	for i, name := range want {
		if fields[i]["name"] != name || fields[i]["inline"] != true {
			t.Fatalf("field %d: %+v", i, fields[i])
		}
	}
}

func TestFailEmbedStyle(t *testing.T) {
	embed := failEmbed(CheckoutInfo{
		Error:     `[{"code":"U103","message":"We were unable to process your credit card"}]`,
		Store:     "US",
		EventID:   "07006348AEEB3AAA",
		Account:   "acct@example.com",
		Profile:   "profile@example.com",
		CardLast4: "4242",
	})
	if embed["title"] != "Payment failed" {
		t.Fatalf("title: %v", embed["title"])
	}
	desc, _ := embed["description"].(string)
	if desc[:16] != "Checkout failed:" {
		t.Fatalf("description: %q", desc)
	}
	fields := embed["fields"].([]map[string]any)
	if len(fields) != 5 {
		t.Fatalf("fail fields=%d want 5", len(fields))
	}
}
