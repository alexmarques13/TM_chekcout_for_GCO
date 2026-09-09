package tm

import "testing"

func TestPickDeclineQuote(t *testing.T) {
	root := map[string]any{
		"data": map[string]any{
			"insuranceQuotes": map[string]any{
				"quotes": []any{
					map[string]any{
						"quoteId":    "yes-id",
						"provider":   "allianz",
						"variantId":  "1351461##FEAT_YES##false",
						"isDecline":  false,
						"price":      map[string]any{"subCurrencyValue": 1275.0},
					},
					map[string]any{
						"quoteId":   "no-id",
						"provider":  "allianz",
						"variantId": "1351461##FEAT_NO##true",
						"isDecline": true,
					},
				},
			},
		},
	}
	sel, ok := pickDeclineQuote(root)
	if !ok {
		t.Fatal("expected decline quote")
	}
	if sel.QuoteID != "no-id" {
		t.Fatalf("quoteId=%q", sel.QuoteID)
	}
	field := sel.PurchaseField()
	if field["optIn"] != false {
		t.Fatalf("optIn=%v", field["optIn"])
	}
	if _, hasPrice := field["price"]; hasPrice {
		t.Fatal("decline must not include price")
	}
}
