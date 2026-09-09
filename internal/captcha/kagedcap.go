package captcha

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	checkoutSiteKey   = "6LczUHQrAAAAAGB50LJU2UVRDrXQECmGuyPXUvX9"
	defaultKagedCapURL = "https://api.kagedcap.io/solve"
	defaultUserAgent  = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36"
)

type Client struct {
	apiKey    string
	proxy     string
	userAgent string
	http      *http.Client
	solveURL  string
}

func New(apiKey string) *Client {
	solveURL := strings.TrimSpace(os.Getenv("KAGEDCAP_API_URL"))
	if solveURL == "" {
		solveURL = defaultKagedCapURL
	}
	return &Client{
		apiKey:    strings.TrimSpace(apiKey),
		solveURL:  solveURL,
		userAgent: defaultUserAgent,
		http:      &http.Client{Timeout: 120 * time.Second},
	}
}

func (k *Client) WithProxy(proxy string) *Client {
	k.proxy = strings.TrimSpace(proxy)
	return k
}

func (k *Client) SolveCheckoutRecaptcha(checkoutPageURL string) (string, error) {
	return k.SolveRecaptchaEnterprise(checkoutPageURL, checkoutSiteKey, "submit")
}

func (k *Client) SolveRecaptchaEnterprise(pageURL, siteKey, action string) (string, error) {
	if k.apiKey == "" {
		return "", fmt.Errorf("KagedCap API key is empty")
	}

	task := "ReCaptchaV3EnterpriseTaskProxyLess"
	body := map[string]any{
		"task":      task,
		"url":       pageURL,
		"sitekey":   siteKey,
		"action":    action,
		"userAgent": k.userAgent,
	}
	if k.proxy != "" {
		if p, err := toHTTPProxy(k.proxy); err == nil {
			body["proxy"] = p
			body["task"] = "ReCaptchaV3EnterpriseTask"
		}
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, k.solveURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", k.apiKey)

	resp, err := k.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("kagedcap request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("kagedcap HTTP %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var data struct {
		Success bool    `json:"success"`
		Token   string  `json:"token"`
		Score   float64 `json:"score"`
		Message string  `json:"message"`
		Code    string  `json:"code"`
	}
	if err := json.Unmarshal(respBody, &data); err != nil {
		return "", fmt.Errorf("kagedcap decode: %w", err)
	}
	if !data.Success || data.Token == "" {
		msg := data.Message
		if msg == "" {
			msg = string(respBody)
		}
		if data.Code != "" {
			msg = data.Code + ": " + msg
		}
		return "", fmt.Errorf("kagedcap solve failed: %s", truncate(msg, 200))
	}
	return data.Token, nil
}

func toHTTPProxy(line string) (string, error) {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
		return line, nil
	}
	parts := strings.Split(line, ":")
	if len(parts) < 4 {
		return "", fmt.Errorf("bad proxy format")
	}
	host := parts[0]
	port := parts[1]
	user := parts[2]
	pass := strings.Join(parts[3:], ":")
	return fmt.Sprintf("http://%s:%s@%s:%s", user, pass, host, port), nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
