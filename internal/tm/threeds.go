package tm

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	http "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	braintreeGraphQL = "https://payments.braintree-api.com/graphql"
	braintree3DS     = "https://api.braintreegateway.com/merchants/%s/client_api/v1/payment_methods/%s/three_d_secure/lookup"
)

type ThreeDSFlow struct {
	client    tlsclient.HttpClient
	userAgent string
	sessionID string
}

func NewThreeDSFlow(userAgent, proxyURL string) (*ThreeDSFlow, error) {
	opts := []tlsclient.HttpClientOption{
		tlsclient.WithTimeoutSeconds(90),
		tlsclient.WithClientProfile(profiles.Chrome_133),
	}
	if proxyURL = strings.TrimSpace(proxyURL); proxyURL != "" {
		opts = append(opts, tlsclient.WithProxyUrl(proxyURL))
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), opts...)
	if err != nil {
		return nil, err
	}
	var sid [16]byte
	_, _ = rand.Read(sid[:])
	return &ThreeDSFlow{
		client:    client,
		userAgent: userAgent,
		sessionID: hex.EncodeToString(sid[:]),
	}, nil
}

func (f *ThreeDSFlow) Close() {}

func (f *ThreeDSFlow) do(req *http.Request) (*http.Response, error) {
	if req.Header.Get("user-agent") == "" {
		req.Header.Set("user-agent", f.userAgent)
	}
	return f.client.Do(req)
}

func (f *ThreeDSFlow) braintreeHeaders(authFingerprint string) http.Header {
	h := http.Header{}
	h.Set("accept", "*/*")
	h.Set("accept-language", "en-US,en;q=0.9")
	h.Set("content-type", "application/json")
	h.Set("origin", "https://assets.braintreegateway.com")
	h.Set("referer", "https://assets.braintreegateway.com/")
	h.Set("user-agent", f.userAgent)
	return h
}

func (f *ThreeDSFlow) StartDeviceCollection() error {
	req, _ := http.NewRequest(http.MethodGet, "https://geo.cardinalcommerce.com/DeviceFingerprintWeb/V2/Browser/Render?referenceId=tm&orgUnitId=5f871d1038a6a872a1b3f8d1&threatmetrix=true&tmEventType=PAYMENT&geolocation=false&alias=Default&origin=Braintree", nil)
	req.Header = f.braintreeHeaders("")
	resp, err := f.do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return nil
}

func (f *ThreeDSFlow) fetchCardinalJWT(authFingerprint string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"browserColorDepth": 24,
		"browserJavaEnabled": false,
		"browserJavascriptEnabled": true,
		"browserLanguage": "en-US",
		"browserScreenHeight": 1080,
		"browserScreenWidth": 1920,
		"browserTimeZone": 300,
	})
	req, _ := http.NewRequest(http.MethodPost, "https://api.braintreegateway.com:443/merchants/unknown/client_api/v1/configuration", bytes.NewReader(body))
	req.Header = f.braintreeHeaders(authFingerprint)
	req.Header.Set("authorization", "Bearer "+authFingerprint)
	resp, err := f.do(req)
	if err != nil {
		return "", err
	}
	raw, _ := ioRead(resp)
	var data map[string]any
	if json.Unmarshal(raw, &data) != nil {
		return authFingerprint, nil
	}
	if jwt := strVal(nestedAny(data, "threeDSecure", "cardinalAuthenticationJWT")); jwt != "" {
		return jwt, nil
	}
	return authFingerprint, nil
}

func (f *ThreeDSFlow) collectCardinalFingerprint(jwt, cardBin string) (string, error) {
	ref := f.sessionID
	if len(cardBin) >= 6 {
		ref = cardBin + ref
	}
	form := url.Values{
		"Bin":          {cardBin},
		"JWT":          {jwt},
		"ReferenceId":  {ref},
		"OrgUnitId":    {"5f871d1038a6a872a1b3f8d1"},
	}
	req, _ := http.NewRequest(http.MethodPost, "https://geo.cardinalcommerce.com/DeviceFingerprintWeb/V2/Browser/SaveBrowserData", strings.NewReader(form.Encode()))
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("user-agent", f.userAgent)
	resp, err := f.do(req)
	if err != nil {
		return ref, nil
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return ref, nil
}

func (f *ThreeDSFlow) threeDSecureLookup(authFingerprint, providerNonce string, amount float64, cardBin, dfReferenceID string) (map[string]any, error) {
	body, _ := json.Marshal(map[string]any{
		"amount": amount,
		"additionalInfo": map[string]any{
			"acsWindowSize": "03",
			"billingLine1":  "",
			"dfReferenceId": dfReferenceID,
		},
		"bin": cardBin,
	})
	lookupURL := fmt.Sprintf("https://api.braintreegateway.com:443/merchants/unknown/client_api/v1/payment_methods/%s/three_d_secure/lookup", url.PathEscape(providerNonce))
	req, _ := http.NewRequest(http.MethodPost, lookupURL, bytes.NewReader(body))
	req.Header = f.braintreeHeaders(authFingerprint)
	req.Header.Set("authorization", "Bearer "+authFingerprint)
	req.Header.Set("content-type", "application/json")
	resp, err := f.do(req)
	if err != nil {
		return nil, err
	}
	raw, _ := ioRead(resp)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("3DS lookup HTTP %d: %s", resp.StatusCode, truncate(string(raw), 200))
	}
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ioRead(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
