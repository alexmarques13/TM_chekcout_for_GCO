package tm

import (
	"fmt"
	"strings"
)

type SeatOffer struct {
	OfferID        string
	OfferName      string
	PriceLevelID   string
	PriceZoneNum   int
	FaceValue      int
	ServiceCharge  int
	FacilityCharge int
	Currency       string
}

func (o SeatOffer) Valid() bool {
	return strings.TrimSpace(o.OfferID) != ""
}

func (o SeatOffer) Total() int {
	total := o.FaceValue + o.ServiceCharge + o.FacilityCharge
	if total <= 0 {
		return o.FaceValue
	}
	return total
}

func strVal(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case fmt.Stringer:
		return t.String()
	default:
		return fmt.Sprint(v)
	}
}

func nestedAny(data map[string]any, keys ...string) any {
	cur := any(data)
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func nestedMap(data map[string]any, keys ...string) map[string]any {
	v := nestedAny(data, keys...)
	m, _ := v.(map[string]any)
	return m
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func isUsableCheckoutSessionID(id string) bool {
	return IsValidCheckoutSessionID(id) || IsEmbeddedCheckoutSessionID(id)
}

func isValidCheckoutSessionID(id string) bool {
	return isUsableCheckoutSessionID(id)
}
