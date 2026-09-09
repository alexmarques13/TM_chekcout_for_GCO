package tm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	http "github.com/bogdanfinn/fhttp"
)

const insuranceGQL = "https://app.ticketmaster.com/spons-upsell-gql"

const queryInsuranceQuotes = `
mutation insuranceQuotes($sessionID: String, $eventID: String, $quoteRequest: QuoteRequest) {
  insuranceQuotes(sessionID: $sessionID, eventID: $eventID, quoteRequest: $quoteRequest) {
    quotes {
      quoteId
      provider
      variantId
      isDecline
      price {
        currencyCode
        currencyValue
        subCurrencyValue
      }
    }
  }
}`

type InsuranceSelection struct {
	QuoteID   string
	Provider  string
	VariantID string
}

func (s *Session) DeclineInsurance(reserveID, eventID string, ticketUSD float64, postalCode string) map[string]any {
	sel := InsuranceSelection{}
	if quote, err := s.fetchDeclineInsuranceQuote(reserveID, eventID, ticketUSD, postalCode); err != nil {
		log.Printf("insurance quote: %v — declining without quote ids", err)
	} else {
		sel = quote
	}
	return sel.PurchaseField()
}

func (sel InsuranceSelection) PurchaseField() map[string]any {
	out := map[string]any{"optIn": false}
	if sel.QuoteID != "" {
		out["quoteId"] = sel.QuoteID
	}
	if sel.Provider != "" {
		out["provider"] = sel.Provider
	}
	if sel.VariantID != "" {
		out["variantId"] = sel.VariantID
	}
	return out
}

func (s *Session) fetchDeclineInsuranceQuote(reserveID, eventID string, ticketUSD float64, postalCode string) (InsuranceSelection, error) {
	sessionID := s.PurchaseSessionID()
	if sessionID == "" {
		sessionID = reserveID
	}
	payload := map[string]any{
		"operationName": "insuranceQuotes",
		"query":         queryInsuranceQuotes,
		"variables": map[string]any{
			"sessionID": sessionID,
			"eventID":   eventID,
			"quoteRequest": map[string]any{
				"currencyCode":    "USD",
				"numberOfTickets": 1,
				"ticketPrice":     ticketUSD,
				"postalCode":      postalCode,
				"countryCode":     "US",
				"requestId":       reserveID,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return InsuranceSelection{}, err
	}
	req, err := http.NewRequest(http.MethodPost, insuranceGQL, bytes.NewReader(body))
	if err != nil {
		return InsuranceSelection{}, err
	}
	req.Header.Set("accept", "*/*")
	req.Header.Set("content-type", "application/json")
	req.Header.Set("origin", CheckoutSDK)
	if s.GraphQLReferer != "" {
		req.Header.Set("referer", s.GraphQLReferer)
	} else {
		req.Header.Set("referer", CheckoutSDK+"/")
	}
	req.Header.Set("user-agent", s.Config.UserAgent)
	s.attachCheckoutCookies(req)

	resp, err := s.DoMain(req)
	if err != nil {
		return InsuranceSelection{}, err
	}
	raw, err := ReadBody(resp)
	if err != nil {
		return InsuranceSelection{}, err
	}
	if resp.StatusCode >= 400 {
		return InsuranceSelection{}, fmt.Errorf("insuranceQuotes HTTP %d: %s", resp.StatusCode, truncBody(raw))
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return InsuranceSelection{}, fmt.Errorf("insuranceQuotes decode: %w", err)
	}
	sel, ok := pickDeclineQuote(data)
	if !ok {
		return InsuranceSelection{}, fmt.Errorf("no decline insurance quote in response")
	}
	return sel, nil
}

func pickDeclineQuote(root map[string]any) (InsuranceSelection, bool) {
	var quotes []any
	if q, ok := nestedAny(root, "data", "insuranceQuotes", "quotes").([]any); ok {
		quotes = q
	}
	if len(quotes) == 0 {
		walkJSON(root, func(key string, val any) {
			if key != "quotes" {
				return
			}
			if list, ok := val.([]any); ok && len(quotes) == 0 {
				quotes = list
			}
		})
	}
	for _, item := range quotes {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		declined, _ := m["isDecline"].(bool)
		if !declined {
			continue
		}
		sel := InsuranceSelection{
			QuoteID:   strVal(m["quoteId"]),
			Provider:  strVal(m["provider"]),
			VariantID: strVal(m["variantId"]),
		}
		if sel.QuoteID == "" {
			sel.QuoteID = strVal(m["quoteID"])
		}
		if sel.Provider == "" {
			sel.Provider = "allianz"
		}
		return sel, true
	}
	return InsuranceSelection{}, false
}

func truncBody(raw []byte) string {
	s := string(raw)
	if len(s) > 240 {
		return s[:240]
	}
	return s
}
