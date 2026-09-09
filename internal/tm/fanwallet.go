package tm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"

	http "github.com/bogdanfinn/fhttp"

	"tm_checkout/internal/config"
)

const (
	FanWalletBase   = "https://payment.ticketmaster.com"
	fanWalletAPIKey = "d2ViLnRpY2tldG1hc3Rlci5wcmltYXJ5"
)

type FanWalletClient struct {
	session       *Session
	fingerprint   string
	correlationID string
	referer       string
}

func NewFanWalletClient(session *Session, fingerprint, reserveID, referer string) *FanWalletClient {
	return &FanWalletClient{
		session:       session,
		fingerprint:   fingerprint,
		correlationID: fmt.Sprintf("checkout-2-%s-%s", session.CorrelationID, reserveID),
		referer:       referer,
	}
}

func (f *FanWalletClient) CreateNonce(cardToken string, billing config.BillingProfile) (string, error) {
	payload := map[string]any{
		"funding_source": "creditcard",
		"funding_source_details": map[string]any{
			"clawback": false,
			"customer_details": map[string]any{
				"billing_address": map[string]string{
					"address_line1": billing.AddressLine,
					"city":          billing.City,
					"country":       billing.Country,
					"name_on_card":  billing.HolderName,
					"phone":         billing.Phone,
					"postal_code":   billing.PostalCode,
					"state":         billing.State,
				},
			},
			"default_payment_instrument": false,
			"instrument_token":         cardToken,
			"instrument_token_source":  "BRAINTREE",
			"save_to_wallet":           false,
		},
	}
	q := url.Values{
		"apikey":      {fanWalletAPIKey},
		"clawback":    {"false"},
		"fingerprint": {f.fingerprint},
	}
	var resp map[string]any
	if err := f.do(http.MethodPost, "/payment/v1/nonces?"+q.Encode(), payload, &resp); err != nil {
		return "", err
	}
	nonce, _ := resp["nonce"].(string)
	if nonce == "" {
		return "", fmt.Errorf("FanWallet nonce missing")
	}
	return nonce, nil
}

func (f *FanWalletClient) AttachCVV(cvvToken, tmNonce string) error {
	q := url.Values{
		"apikey":      {fanWalletAPIKey},
		"fingerprint": {f.fingerprint},
	}
	payload := map[string]string{
		"instrument_token": cvvToken,
		"nonce":            tmNonce,
	}
	return f.do(http.MethodPut, "/payment/v1/cvv?"+q.Encode(), payload, nil)
}

func (f *FanWalletClient) ProviderNonce(tmNonce, reserveID string) (string, bool, error) {
	q := url.Values{
		"apiKey":                {fanWalletAPIKey},
		"fingerprint":           {f.fingerprint},
		"required_3ds":          {"false"},
		"transaction_reference": {reserveID},
	}
	var resp map[string]any
	path := fmt.Sprintf("/payment/v1/nonces/braintree/%s?%s", url.PathEscape(tmNonce), q.Encode())
	if err := f.do(http.MethodGet, path, nil, &resp); err != nil {
		return "", false, err
	}
	providerNonce, _ := resp["provider_nonce"].(string)
	if providerNonce == "" {
		return "", false, fmt.Errorf("FanWallet missing provider_nonce")
	}
	bypass, _ := resp["bypass"].(bool)
	return providerNonce, bypass, nil
}

func (f *FanWalletClient) Patch3DS(tmNonce, reserveID, authID, provider3DSNonce string) error {
	q := url.Values{
		"apiKey":                {fanWalletAPIKey},
		"fingerprint":           {f.fingerprint},
		"required_3ds":          {"false"},
		"transaction_reference": {reserveID},
	}
	payload := map[string]string{
		"authentication_3ds_id": authID,
		"provider_3ds_nonce":    provider3DSNonce,
	}
	path := fmt.Sprintf("/payment/v1/nonces/%s?%s", url.PathEscape(tmNonce), q.Encode())
	return f.do(http.MethodPatch, path, payload, nil)
}

func (f *FanWalletClient) do(method, path string, payload any, out any) error {
	var body io.Reader
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, FanWalletBase+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("accept", "application/json")
	req.Header.Set("content-type", "application/json;charset=UTF-8")
	req.Header.Set("origin", CheckoutSDK)
	req.Header.Set("referer", f.referer+"/")
	req.Header.Set("user-agent", f.session.Config.UserAgent)
	req.Header.Set("tmps-correlation-id", f.correlationID)
	f.session.attachCheckoutCookies(req)

	resp, err := f.session.DoMain(req)
	if err != nil {
		return err
	}
	raw, err := ReadBody(resp)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		text := string(raw)
		if len(text) > 500 {
			text = text[:500]
		}
		return fmt.Errorf("FanWallet %s HTTP %d: %s", path, resp.StatusCode, text)
	}
	if out == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}
