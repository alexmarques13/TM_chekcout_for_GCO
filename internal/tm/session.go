package tm

import (
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	http "github.com/bogdanfinn/fhttp"
	tlsclient "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"

	"tm_checkout/internal/api"
)

const WWW = "https://www.ticketmaster.com"

type SessionConfig struct {
	ProxyURL      string
	UserAgent     string
	SystemEventID string
}

type Session struct {
	Config         SessionConfig
	client         tlsclient.HttpClient
	CorrelationID  string
	GCOSessionID   string
	GraphQLReferer string
	checkoutRegion string
	monetateID     string
	KasadaHeaders  map[string]string
}

func NewSession(rawProxy string, cookies []api.Cookie) (*Session, error) {
	jar := tlsclient.NewCookieJar()
	opts := []tlsclient.HttpClientOption{
		tlsclient.WithTimeoutSeconds(120),
		tlsclient.WithClientProfile(profiles.Chrome_133),
		tlsclient.WithCookieJar(jar),
	}
	proxyURL, err := formatProxyURL(rawProxy)
	if err != nil {
		return nil, err
	}
	if proxyURL != "" {
		opts = append(opts, tlsclient.WithProxyUrl(proxyURL))
	}
	client, err := tlsclient.NewHttpClient(tlsclient.NewNoopLogger(), opts...)
	if err != nil {
		return nil, err
	}
	s := &Session{
		Config: SessionConfig{
			ProxyURL:  proxyURL,
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/133.0.0.0 Safari/537.36",
		},
		client:        client,
		CorrelationID: fmt.Sprintf("%d", time.Now().UnixNano()),
		KasadaHeaders: map[string]string{},
	}
	s.seedCookies(cookies)
	return s, nil
}

func (s *Session) seedCookies(cookies []api.Cookie) {
	for _, c := range cookies {
		if c.Name == "" || c.Value == "" {
			continue
		}
		domain := strings.TrimSpace(c.Domain)
		if domain == "" {
			domain = ".ticketmaster.com"
		}
		if !strings.HasPrefix(domain, ".") && !strings.Contains(domain, "ticketmaster.com") {
			domain = ".ticketmaster.com"
		}
		u := &url.URL{Scheme: "https", Host: strings.TrimPrefix(domain, ".")}
		s.client.SetCookies(u, []*http.Cookie{{
			Name:   c.Name,
			Value:  c.Value,
			Path:   firstNonEmpty(c.Path, "/"),
			Domain: domain,
			Secure: c.Secure,
		}})
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *Session) Close() {}

func (s *Session) DoMain(req *http.Request) (*http.Response, error) {
	if req.Header.Get("user-agent") == "" {
		req.Header.Set("user-agent", s.Config.UserAgent)
	}
	return s.client.Do(req)
}

func (s *Session) IdentityCookies() []api.Cookie {
	var out []api.Cookie
	seen := map[string]bool{}
	for _, raw := range []string{WWW, CheckoutSDK, "https://auth.ticketmaster.com", "https://identity.ticketmaster.com"} {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		for _, ck := range s.client.GetCookies(u) {
			key := ck.Name + "=" + ck.Value
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, api.Cookie{Name: ck.Name, Value: ck.Value, Domain: ck.Domain})
		}
	}
	return out
}

func (s *Session) CookieValue(name string) string {
	for _, u := range []*url.URL{
		mustURL(WWW),
		mustURL(CheckoutSDK),
	} {
		for _, ck := range s.client.GetCookies(u) {
			if ck.Name == name {
				return ck.Value
			}
		}
	}
	return ""
}

func mustURL(raw string) *url.URL {
	u, _ := url.Parse(raw)
	return u
}

func (s *Session) WWWHeaders(referer string) http.Header {
	h := http.Header{}
	h.Set("accept-language", "en-US,en;q=0.9")
	h.Set("user-agent", s.Config.UserAgent)
	h.Set("sec-ch-ua", `"Google Chrome";v="133", "Chromium";v="133", "Not_A Brand";v="24"`)
	h.Set("sec-ch-ua-mobile", "?0")
	h.Set("sec-ch-ua-platform", `"Windows"`)
	if referer != "" {
		h.Set("referer", referer)
	}
	return h
}

func (s *Session) applyKasadaHeaders(h http.Header) {
	for k, v := range s.KasadaHeaders {
		if v != "" {
			h.Set(k, v)
		}
	}
}

func (s *Session) PurchaseSessionID() string {
	if sid := strings.TrimSpace(s.GCOSessionID); isUsableCheckoutSessionID(sid) {
		return sid
	}
	return ""
}

func (s *Session) ResetPurchaseSession() {
	if isUsableCheckoutSessionID(s.GCOSessionID) {
		return
	}
	s.GCOSessionID = ""
}

func (s *Session) PurchaseSessionIDOrResolve(reserveID, eventURL string) (string, error) {
	if sid := s.PurchaseSessionID(); sid != "" {
		return sid, nil
	}
	if sid := checkoutSessionFromCookies(s); sid != "" {
		s.GCOSessionID = sid
		return sid, nil
	}
	return s.resolveCheckoutSession(reserveID, eventURL)
}

func (s *Session) resolvePurchaseSessionID(reserveID, eventURL string) (string, error) {
	return s.PurchaseSessionIDOrResolve(reserveID, eventURL)
}

func (s *Session) captureCheckoutSessionHeader(resp *http.Response) {
	if resp == nil {
		return
	}
	for _, key := range []string{"tmps-session-id", "Tmps-Session-Id"} {
		if sid := strings.TrimSpace(resp.Header.Get(key)); isUsableCheckoutSessionID(sid) {
			s.GCOSessionID = sid
			return
		}
	}
}

func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func checkoutSDKHeaders(s *Session) http.Header {
	h := http.Header{}
	h.Set("accept", "*/*")
	h.Set("user-agent", s.Config.UserAgent)
	return h
}

func ParseProxy(raw string) (server, user, pass string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("empty proxy")
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid proxy format")
	}
	server = fmt.Sprintf("http://%s:%s", parts[0], parts[1])
	if len(parts) >= 4 {
		user = parts[2]
		pass = strings.Join(parts[3:], ":")
	}
	return server, user, pass, nil
}

func formatProxyURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw, nil
	}
	server, user, pass, err := ParseProxy(raw)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(server)
	if err != nil {
		return "", err
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	return u.String(), nil
}
