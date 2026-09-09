package tm

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

type PaymentClientToken struct {
	Fingerprint   string
	BraintreeAuth string
	MerchantID    string
}

func ParsePaymentClientToken(raw string) (*PaymentClientToken, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty client token")
	}

	decoded, err := decodeBase64Flexible(raw)
	if err != nil {
		return nil, fmt.Errorf("decode client token: %w", err)
	}

	var outer map[string]any
	if err := json.Unmarshal(decoded, &outer); err != nil {
		return nil, fmt.Errorf("parse client token json: %w", err)
	}

	info := &PaymentClientToken{}

	if fp, ok := outer["fingerprint"].(string); ok && fp != "" {
		info.Fingerprint = fp
	}

	if providers, ok := outer["token_providers"].([]any); ok {
		for _, item := range providers {
			provider, _ := item.(map[string]any)
			if provider == nil {
				continue
			}
			id, _ := provider["provider_id"].(string)
			if !strings.EqualFold(id, "BRAINTREE") {
				continue
			}
			clientToken, _ := provider["client_token"].(string)
			if clientToken == "" {
				continue
			}
			btDecoded, err := decodeBase64Flexible(clientToken)
			if err != nil {
				return nil, fmt.Errorf("decode braintree client token: %w", err)
			}
			var bt map[string]any
			if err := json.Unmarshal(btDecoded, &bt); err != nil {
				return nil, fmt.Errorf("parse braintree client token: %w", err)
			}
			if auth, ok := bt["authorizationFingerprint"].(string); ok && auth != "" {
				info.BraintreeAuth = auth
			}
			if mid, ok := bt["merchantId"].(string); ok && mid != "" {
				info.MerchantID = mid
			}
			break
		}
	}

	if info.BraintreeAuth == "" {
		if auth, ok := outer["authorizationFingerprint"].(string); ok && auth != "" {
			info.BraintreeAuth = auth
		}
	}
	if info.Fingerprint == "" {
		if key, ok := outer["tokenizationKey"].(string); ok && key != "" {
			info.Fingerprint = key
		}
	}
	if info.BraintreeAuth == "" {
		if key, ok := outer["tokenizationKey"].(string); ok && key != "" {
			info.BraintreeAuth = key
		}
	}

	if info.Fingerprint == "" {
		return nil, fmt.Errorf("client token missing fingerprint")
	}
	if info.BraintreeAuth == "" {
		return nil, fmt.Errorf("client token missing braintree authorization fingerprint")
	}
	return info, nil
}

func authorizationFingerprint(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty authorization fingerprint")
	}
	if strings.Count(raw, ".") >= 2 {
		return raw, nil
	}
	decoded, err := decodeBase64Flexible(raw)
	if err != nil {
		return raw, nil
	}
	var inner struct {
		AuthorizationFingerprint string `json:"authorizationFingerprint"`
	}
	if json.Unmarshal(decoded, &inner) == nil && inner.AuthorizationFingerprint != "" {
		return inner.AuthorizationFingerprint, nil
	}
	return raw, nil
}

func decodeBase64Flexible(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if decoded, err := base64.RawURLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.URLEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return decoded, nil
	}
	padded := raw + strings.Repeat("=", (-len(raw)%4+4)%4)
	if decoded, err := base64.URLEncoding.DecodeString(padded); err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(padded)
}
