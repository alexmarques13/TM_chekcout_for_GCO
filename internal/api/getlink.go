package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"tm_checkout/internal/log"
)

type Cookie struct {
	Name   string `json:"Name"`
	Value  string `json:"Value"`
	Domain string `json:"Domain"`
	Path   string `json:"Path"`
	Secure bool   `json:"Secure"`
}

type Session struct {
	CheckoutURL string
	Proxy       string
	Cookies     []Cookie
	TaskID      string
	Instance    string
}

func FetchSession(linkURL string) (*Session, error) {
	linkURL = strings.TrimSpace(linkURL)
	if linkURL == "" {
		return nil, fmt.Errorf("link URL is required")
	}

	log.Request("GET", linkURL)
	start := time.Now()

	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(linkURL)
	if err != nil {
		log.Fail(fmt.Sprintf("getLink: %v", err))
		return nil, fmt.Errorf("fetch getLink: %w", err)
	}
	defer resp.Body.Close()

	location := resp.Header.Get("Location")
	log.Response("GET", linkURL, resp.StatusCode, fmt.Sprintf("%dms location=%s", time.Since(start).Milliseconds(), trunc(location, 80)))

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("expected redirect from getLink, got HTTP %d", resp.StatusCode)
	}
	if location == "" {
		return nil, fmt.Errorf("getLink redirect missing Location header")
	}

	parsed, err := url.Parse(location)
	if err != nil {
		return nil, fmt.Errorf("parse redirect: %w", err)
	}

	q := parsed.Query()
	checkoutURL := q.Get("c")
	proxy := q.Get("proxy")
	cookieRaw := q.Get("cookie")

	if checkoutURL == "" {
		return nil, fmt.Errorf("missing checkout URL in redirect")
	}
	if proxy == "" {
		return nil, fmt.Errorf("missing proxy in redirect")
	}
	if cookieRaw == "" {
		return nil, fmt.Errorf("missing cookies in redirect")
	}

	var cookies []Cookie
	if err := json.Unmarshal([]byte(cookieRaw), &cookies); err != nil {
		return nil, fmt.Errorf("parse cookies JSON: %w", err)
	}

	log.Action(fmt.Sprintf("parsed checkout=%s cookies=%d proxy=%s", trunc(checkoutURL, 60), len(cookies), truncProxy(proxy)))

	return &Session{
		CheckoutURL: checkoutURL,
		Proxy:       proxy,
		Cookies:     cookies,
		TaskID:      q.Get("taskId"),
		Instance:    q.Get("instance"),
	}, nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func truncProxy(p string) string {
	parts := strings.Split(p, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1] + ":***"
	}
	return p
}
