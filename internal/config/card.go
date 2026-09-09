package config

import (
	"strings"

	"tm_checkout/internal/cards"
)

type CardConfig struct {
	Number string
	ExpMonth string
	ExpYear  string
	CVV     string
}

type BillingProfile struct {
	HolderName  string
	AddressLine string
	City        string
	State       string
	PostalCode  string
	Country     string
	Phone       string
}

func CardFromCSV(c cards.Card) CardConfig {
	return CardConfig{
		Number:   c.Number,
		ExpMonth: c.ExpMonth,
		ExpYear:  c.ExpYear,
		CVV:      c.CVV,
	}
}

func BillingFromCSV(c cards.Card) BillingProfile {
	return BillingProfile{
		HolderName:  c.Name,
		AddressLine: c.Address,
		City:        c.City,
		State:       c.State,
		PostalCode:  c.Zip,
		Country:     "US",
		Phone:       digitsOnly(c.Phone),
	}
}

func DefaultBilling() BillingProfile {
	return BillingProfile{Country: "US"}
}

func (b BillingProfile) Normalized() BillingProfile {
	out := b
	out.Phone = digitsOnly(out.Phone)
	if out.Country == "" {
		out.Country = "US"
	}
	return out
}

func (c CardConfig) ExpiryMonth() int {
	n := 0
	for _, ch := range strings.TrimSpace(c.ExpMonth) {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func (c CardConfig) ExpiryYear() int {
	s := strings.TrimSpace(c.ExpYear)
	if len(s) == 2 {
		s = "20" + s
	}
	n := 0
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		}
	}
	return n
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		}
	}
	return b.String()
}
