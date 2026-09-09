package tm

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestParsePaymentClientTokenTMFormat(t *testing.T) {
	outer := map[string]any{
		"fingerprint": "fanwallet-fp",
		"token_providers": []map[string]any{{
			"provider_id": "BRAINTREE",
			"client_token": base64.StdEncoding.EncodeToString([]byte(`{"authorizationFingerprint":"bt-auth","merchantId":"merchant123"}`)),
		}},
	}
	raw, _ := json.Marshal(outer)
	token := base64.StdEncoding.EncodeToString(raw)

	info, err := ParsePaymentClientToken(token)
	if err != nil {
		t.Fatalf("ParsePaymentClientToken: %v", err)
	}
	if info.Fingerprint != "fanwallet-fp" {
		t.Fatalf("fingerprint=%q", info.Fingerprint)
	}
	if info.BraintreeAuth != "bt-auth" {
		t.Fatalf("braintree auth=%q", info.BraintreeAuth)
	}
}
