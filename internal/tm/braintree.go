package tm

import (
	"bytes"
	"encoding/json"
	"fmt"

	http "github.com/bogdanfinn/fhttp"

	"tm_checkout/internal/config"
)

const braintreeTokenizeQuery = `
mutation TokenizeCreditCard($input: TokenizeCreditCardInput!) {
  tokenizeCreditCard(input: $input) {
    token
    creditCard {
      bin
      brandCode
      last4
      expirationMonth
      expirationYear
    }
  }
}`

type BraintreeTokenizer struct {
	flow *ThreeDSFlow
}

func NewBraintreeTokenizer(userAgent, proxyURL string) (*BraintreeTokenizer, error) {
	flow, err := NewThreeDSFlow(userAgent, proxyURL)
	if err != nil {
		return nil, err
	}
	return &BraintreeTokenizer{flow: flow}, nil
}

func (b *BraintreeTokenizer) Close() {
	if b.flow != nil {
		b.flow.Close()
	}
}

func (b *BraintreeTokenizer) TokenizeCard(authFingerprint string, card config.CardConfig) (string, error) {
	variables := map[string]any{
		"input": map[string]any{
			"creditCard": map[string]any{
				"number":          card.Number,
				"expirationMonth": fmt.Sprintf("%02d", card.ExpiryMonth()),
				"expirationYear":  fmt.Sprintf("%d", card.ExpiryYear()),
			},
			"options": map[string]any{"validate": false},
		},
	}
	return b.tokenize(authFingerprint, variables)
}

func (b *BraintreeTokenizer) TokenizeCVV(authFingerprint, cvv string) (string, error) {
	variables := map[string]any{
		"input": map[string]any{
			"creditCard": map[string]any{"cvv": cvv},
			"options":    map[string]any{"validate": false},
		},
	}
	return b.tokenize(authFingerprint, variables)
}

func (b *BraintreeTokenizer) tokenize(authFingerprint string, variables map[string]any) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"clientSdkMetadata": map[string]string{
			"source": "client", "integration": "custom", "sessionId": b.flow.sessionID,
		},
		"query":         braintreeTokenizeQuery,
		"variables":     variables,
		"operationName": "TokenizeCreditCard",
	})
	req, _ := http.NewRequest(http.MethodPost, braintreeGraphQL, bytes.NewReader(body))
	req.Header = b.flow.braintreeHeaders(authFingerprint)
	req.Header.Set("authorization", "Bearer "+authFingerprint)
	req.Header.Set("braintree-version", "2018-05-10")

	resp, err := b.flow.do(req)
	if err != nil {
		return "", err
	}
	raw, _ := ioRead(resp)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Braintree tokenize HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", err
	}
	token := strVal(nestedAny(payload, "data", "tokenizeCreditCard", "token"))
	if token == "" {
		return "", fmt.Errorf("Braintree tokenize missing token")
	}
	return token, nil
}

func (b *BraintreeTokenizer) Lookup3DS(authFingerprint, providerNonce string, amountDollars float64, cardBin string) (authID, provider3DSNonce string, softFail bool, err error) {
	if len(cardBin) > 6 {
		cardBin = cardBin[:6]
	}
	if err := b.flow.StartDeviceCollection(); err != nil {
		return "", "", false, err
	}
	jwt, err := b.flow.fetchCardinalJWT(authFingerprint)
	if err != nil {
		return "", "", false, err
	}
	dfReferenceID, err := b.flow.collectCardinalFingerprint(jwt, cardBin)
	if err != nil {
		return "", "", false, err
	}
	lookup, err := b.flow.threeDSecureLookup(authFingerprint, providerNonce, amountDollars, cardBin, dfReferenceID)
	if err != nil {
		return "", "", true, nil
	}
	pm, _ := lookup["paymentMethod"].(map[string]any)
	info, _ := pm["threeDSecureInfo"].(map[string]any)
	status := strVal(info["status"])
	softFail = status == "lookup_error" || status == "unsupported_card"
	authID = strVal(info["threeDSecureAuthenticationId"])
	if authID == "" {
		if lookupSub, ok := lookup["lookup"].(map[string]any); ok {
			authID = strVal(lookupSub["md"])
		}
	}
	provider3DSNonce = strVal(pm["nonce"])
	if provider3DSNonce == "" {
		provider3DSNonce = providerNonce
	}
	if acsURL := strVal(info["acsUrl"]); acsURL != "" {
		return "", "", false, fmt.Errorf("3DS challenge required")
	}
	if authID == "" {
		return "", "", softFail, nil
	}
	return authID, provider3DSNonce, softFail, nil
}
