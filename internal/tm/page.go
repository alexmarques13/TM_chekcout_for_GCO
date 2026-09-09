package tm

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var checkoutNextDataRE = regexp.MustCompile(`<script id="__NEXT_DATA__" type="application/json">(.+?)</script>`)

type CheckoutCart struct {
	ReserveID     string
	EventURL      string
	SystemEventID string
	Section       string
	Row           string
	Seat          string
	Account       string
	Profile       string
	Offer         SeatOffer
	QuotedCents   int
}

func ParseCheckoutURL(raw string) (reserveID, eventURL string, err error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", "", err
	}
	path := strings.Trim(u.Path, "/")
	if path != "" {
		reserveID = strings.Split(path, "/")[0]
	}
	eventURL = u.Query().Get("edp")
	if eventURL != "" {
		eventURL, _ = url.QueryUnescape(eventURL)
	}
	if reserveID == "" {
		return "", "", fmt.Errorf("missing reserve id in checkout url")
	}
	return reserveID, eventURL, nil
}

func (s *Session) ParseCheckoutPage(checkoutURL string) (*CheckoutCart, error) {
	reserveID, eventURL, err := ParseCheckoutURL(checkoutURL)
	if err != nil {
		return nil, err
	}
	html, err := s.fetchCheckoutHTML(checkoutURL)
	if err != nil {
		return nil, err
	}
	if err := CheckCheckoutExpired(200, html); err != nil {
		return nil, err
	}
	cart := &CheckoutCart{
		ReserveID: reserveID,
		EventURL:  eventURL,
	}
	if m := checkoutNextDataRE.FindStringSubmatch(html); len(m) > 1 {
		var next map[string]any
		if json.Unmarshal([]byte(m[1]), &next) == nil {
			fillCartFromJSON(cart, next)
		}
	}
	if cart.SystemEventID == "" {
		cart.SystemEventID = extractEventID(eventURL)
	}
	if cart.Section == "" || cart.Offer.OfferID == "" {
		fillCartFromHTML(cart, html)
	}
	if cart.Offer.OfferID == "" {
		return nil, fmt.Errorf("could not parse ticket offer from checkout page")
	}
	if cart.QuotedCents <= 0 {
		cart.QuotedCents = cart.Offer.Total()
	}
	if cart.Offer.FaceValue <= 0 {
		cart.Offer.FaceValue = 3000
	}
	s.Config.SystemEventID = cart.SystemEventID
	return cart, nil
}

func extractEventID(eventURL string) string {
	if eventURL == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(eventURL, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "0") && len(parts[i]) >= 10 {
			return parts[i]
		}
	}
	return ""
}

func fillCartFromJSON(cart *CheckoutCart, root map[string]any) {
	walkJSON(root, func(key string, val any) {
		switch key {
		case "requestId", "reserveId":
			if s, ok := val.(string); ok && cart.ReserveID == "" {
				cart.ReserveID = s
			}
		case "systemEventId", "systemEventID", "eventId":
			if s, ok := val.(string); ok && cart.SystemEventID == "" {
				cart.SystemEventID = s
			}
		case "section":
			if s, ok := val.(string); ok && cart.Section == "" {
				cart.Section = s
			}
		case "row":
			if s, ok := val.(string); ok && cart.Row == "" {
				cart.Row = s
			}
		case "seat", "startSeat":
			if s, ok := val.(string); ok && cart.Seat == "" {
				cart.Seat = s
			}
		case "ticketTypeId", "extTicketType", "offerId", "id":
			if s, ok := val.(string); ok && cart.Offer.OfferID == "" && len(s) > 8 {
				cart.Offer.OfferID = s
			}
		case "priceLevel", "priceLevelId", "priceLevelSecname":
			if s, ok := val.(string); ok && cart.Offer.PriceLevelID == "" {
				cart.Offer.PriceLevelID = s
			}
		case "faceValue", "basePrice":
			if n, ok := toInt(val); ok && cart.Offer.FaceValue == 0 {
				cart.Offer.FaceValue = n
			}
		case "serviceCharge", "facilityCharge":
			if n, ok := toInt(val); ok {
				if key == "serviceCharge" {
					cart.Offer.ServiceCharge = n
				} else {
					cart.Offer.FacilityCharge = n
				}
			}
		case "grandTotal", "orderTotal", "totalPrice", "totalCharge":
			if n, ok := moneyToCents(val); ok && n > cart.QuotedCents {
				cart.QuotedCents = n
			}
		case "email", "userEmail", "emailAddress", "loginEmail", "accountEmail":
			if s, ok := val.(string); ok && strings.Contains(s, "@") {
				if cart.Account == "" {
					cart.Account = s
				} else if cart.Profile == "" && !strings.EqualFold(s, cart.Account) {
					cart.Profile = s
				}
			}
		}
	})
}

func fillCartFromHTML(cart *CheckoutCart, html string) {
	patterns := map[*string][]*regexp.Regexp{
		&cart.Section: {
			regexp.MustCompile(`"section"\s*:\s*"([^"]+)"`),
			regexp.MustCompile(`Section\s+(\d+)`),
		},
		&cart.Row: {regexp.MustCompile(`"row"\s*:\s*"([^"]+)"`)},
		&cart.Seat: {regexp.MustCompile(`"startSeat"\s*:\s*"([^"]+)"`)},
		&cart.Offer.OfferID: {regexp.MustCompile(`"ticketTypeId"\s*:\s*"([^"]+)"`)},
		&cart.Offer.PriceLevelID: {regexp.MustCompile(`"priceLevelSecname"\s*:\s*"([^"]+)"`)},
	}
	for dst, res := range patterns {
		if strings.TrimSpace(*dst) != "" {
			continue
		}
		for _, re := range res {
			if m := re.FindStringSubmatch(html); len(m) > 1 {
				*dst = m[1]
				break
			}
		}
	}
}

func walkJSON(v any, fn func(string, any)) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			fn(k, val)
			walkJSON(val, fn)
		}
	case []any:
		for _, item := range t {
			walkJSON(item, fn)
		}
	}
}

func toInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case json.Number:
		n, err := t.Int64()
		return int(n), err == nil
	default:
		return 0, false
	}
}

func moneyToCents(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		if t <= 0 {
			return 0, false
		}
		if t != float64(int(t)) || t < 500 {
			return int(t*100 + 0.5), true
		}
		return int(t), true
	case int:
		if t <= 0 {
			return 0, false
		}
		return t, true
	case json.Number:
		f, err := t.Float64()
		if err != nil || f <= 0 {
			return 0, false
		}
		return moneyToCents(f)
	default:
		return 0, false
	}
}

func parseCheckoutSessionIDRaw(html string) string {
	if sid := parseNextDataSessionID(html); isUsableCheckoutSessionID(sid) {
		return sid
	}
	for _, re := range []*regexp.Regexp{
		checkoutRtfsfeConfigRE,
		checkoutRtfsfeConfigAltRE,
		checkoutRtfsfeSessionRE,
		checkoutReserveSessionRE,
		checkoutTMPSessionRE,
		checkoutRtfsfeSIDRE,
	} {
		if m := re.FindStringSubmatch(html); len(m) > 1 && isUsableCheckoutSessionID(m[1]) {
			return m[1]
		}
	}
	return ""
}

func parseNextDataSessionID(html string) string {
	m := checkoutNextDataRE.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	var next map[string]any
	if json.Unmarshal([]byte(m[1]), &next) != nil {
		return ""
	}
	var found string
	walkJSON(next, func(key string, val any) {
		if key == "sessionId" {
			if s, ok := val.(string); ok && isUsableCheckoutSessionID(s) && found == "" {
				found = s
			}
		}
	})
	return found
}
