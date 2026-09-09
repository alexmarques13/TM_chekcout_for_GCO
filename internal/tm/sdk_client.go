package tm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"regexp"
	"strings"

	http "github.com/bogdanfinn/fhttp"
)

const (
	CheckoutSDK     = "https://checkout.ticketmaster.com"
	CheckoutGraphQL = CheckoutSDK + "/graphql"
)

var (
	checkoutRtfsfeConfigRE    = regexp.MustCompile(`_rtfsfeConfig\s*=\s*Promise\.resolve\(\{"sessionId":"([^"]+)"`)
	checkoutRtfsfeConfigAltRE = regexp.MustCompile(`_rtfsfeConfig\s*=\s*Promise\.resolve\(\{sessionId:"([^"]+)"`)
	checkoutRtfsfeSessionRE   = regexp.MustCompile(`sessionId":"([^"]+)","initialAction":"/checkout"`)
	checkoutReserveSessionRE  = regexp.MustCompile(`"requestId":"[^"]+","sessionId":"([^"]+)"`)
	checkoutTMPSessionRE      = regexp.MustCompile(`"sessionId"\s*:\s*"([0-9][a-zA-Z0-9]{4,7}_[^"]+)"`)
	checkoutRtfsfeSIDRE       = regexp.MustCompile(`_rtfsfeSID\s*=\s*"([^"]+)"`)
)

func (s *Session) SDKCheckoutGraphQL(operation, query, cmd, tmpsGeneric, reserveID string, purchase bool, variables map[string]any) (map[string]any, error) {
	return s.sdkGraphQLWithOrigin(operation, query, variables, cmd, tmpsGeneric, CheckoutSDK, true, reserveID, purchase)
}

func (s *Session) sdkGraphQLWithOrigin(operation, query string, variables map[string]any, cmd, tmpsGeneric, origin string, sameOrigin bool, reserveID string, purchase bool) (map[string]any, error) {
	payload := map[string]any{
		"operationName": operation,
		"variables":     variables,
		"query":         query,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			HumanDelay(800, 1800)
		}
		data, err := s.doSDKGraphQL(operation, body, cmd, tmpsGeneric, origin, sameOrigin, reserveID, purchase)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func (s *Session) doSDKGraphQL(operation string, body []byte, cmd, tmpsGeneric, origin string, sameOrigin bool, reserveID string, purchase bool) (map[string]any, error) {
	req, err := http.NewRequest(http.MethodPost, CheckoutGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range s.sdkGraphQLHeaders(cmd, sameOrigin) {
		req.Header.Set(k, v)
	}
	if sameOrigin {
		for k, v := range s.checkoutSDKHeaders(reserveID) {
			req.Header.Set(k, v)
		}
	}
	if origin != "" {
		req.Header.Set("origin", origin)
	}
	if sameOrigin {
		req.Header.Set("sec-fetch-site", "same-origin")
	} else {
		req.Header.Set("sec-fetch-site", "same-site")
	}
	if tmpsGeneric != "" {
		req.Header.Set("tmps-generic", tmpsGeneric)
	}
	if bid := s.CookieValue("BID"); bid != "" {
		req.Header.Set("x-tm-bid", bid)
	}
	s.applyKasadaHeaders(req.Header)
	s.attachCheckoutCookies(req)

	resp, err := s.DoMain(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	s.captureCheckoutSessionHeader(resp)
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		text := string(raw)
		if len(text) > 500 {
			text = text[:500]
		}
		return nil, fmt.Errorf("SDK GraphQL %s HTTP %d: %s", operation, resp.StatusCode, text)
	}
	var data map[string]any
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("SDK GraphQL %s decode: %w", operation, err)
	}
	if errs, ok := data["errors"]; ok {
		return nil, WrapGraphQLExpired(fmt.Errorf("SDK GraphQL %s failed: %v", operation, errs))
	}
	return data, nil
}

func (s *Session) sdkGraphQLHeaders(cmd string, checkoutOrigin bool) map[string]string {
	origin := WWW
	fetchSite := "same-site"
	if checkoutOrigin {
		origin = CheckoutSDK
		fetchSite = "same-origin"
	}
	h := map[string]string{
		"accept":              "*/*",
		"accept-language":     "en-US,en;q=0.9",
		"content-type":        "application/json",
		"origin":              origin,
		"referer":             s.GraphQLReferer,
		"sec-ch-ua":           `"Google Chrome";v="133", "Chromium";v="133", "Not_A Brand";v="24"`,
		"sec-ch-ua-mobile":    "?0",
		"sec-ch-ua-platform":  `"Windows"`,
		"sec-fetch-dest":      "empty",
		"sec-fetch-mode":      "cors",
		"sec-fetch-site":      fetchSite,
		"tmps-correlation-id": s.CorrelationID,
	}
	if monetate := s.MonetateID(); monetate != "" {
		h["tmps-monetate-id"] = monetate
	}
	if cmd != "" {
		h["x-cmd"] = cmd
		h["x-eid"] = s.Config.SystemEventID
	}
	return h
}

func (s *Session) checkoutSDKHeaders(reserveID string) map[string]string {
	h := map[string]string{}
	corr := s.CorrelationID
	if reserveID != "" {
		corr = corr + "-" + reserveID
	}
	h["tmps-correlation-id"] = corr
	if sid := s.PurchaseSessionID(); sid != "" {
		h["tmps-session-id"] = sid
	}
	h["x-environment-tag"] = checkoutEnvironmentTag(s.CheckoutRegion())
	h["x-tm-domain"] = "TM_US"
	if region := s.CheckoutRegion(); region != "" {
		h["x-region"] = region
	}
	return h
}

func checkoutEnvironmentTag(region string) string {
	if strings.ToLower(region) == "east" {
		return "prode1"
	}
	return "prodw2"
}

func IsValidCheckoutSessionID(id string) bool {
	if !regexp.MustCompile(`^[0-9][a-zA-Z0-9]{4,7}_`).MatchString(id) {
		return false
	}
	return isPlausibleCheckoutSessionID(id)
}

func IsEmbeddedCheckoutSessionID(id string) bool {
	if isFlatCheckoutSessionID(id) {
		return true
	}
	parts := strings.SplitN(id, "_", 2)
	if len(parts) != 2 || len(parts[0]) < 4 || len(parts[1]) < 40 {
		return false
	}
	return isPlausibleCheckoutSessionID(id)
}

func isFlatCheckoutSessionID(id string) bool {
	if len(id) < 40 || len(id) > 128 || strings.Contains(id, "_") {
		return false
	}
	upper := strings.ToUpper(id)
	for _, bad := range []string{"RETINA", "PORTRAIT", "LANDSCAPE"} {
		if strings.Contains(upper, bad) {
			return false
		}
	}
	return true
}

func isPlausibleCheckoutSessionID(id string) bool {
	parts := strings.SplitN(id, "_", 2)
	if len(parts) == 2 && len(parts[1]) < 40 {
		return false
	}
	upper := strings.ToUpper(id)
	for _, bad := range []string{"RETINA", "PORTRAIT", "LANDSCAPE", "DESKTOP", "MOBILE"} {
		if strings.Contains(upper, bad) {
			return false
		}
	}
	return true
}

func (s *Session) MonetateID() string {
	if s.monetateID != "" {
		return s.monetateID
	}
	for _, name := range []string{"mt.v", "mt.v2", "monetateId"} {
		if v := s.CookieValue(name); v != "" {
			s.monetateID = v
			return v
		}
	}
	return ""
}

func (s *Session) CheckoutRegion() string {
	if s.checkoutRegion != "" {
		return s.checkoutRegion
	}
	req, _ := http.NewRequest(http.MethodGet, CheckoutSDK+"/region", nil)
	req.Header = checkoutSDKHeaders(s)
	s.attachCheckoutCookies(req)
	resp, err := s.DoMain(req)
	if err != nil {
		s.checkoutRegion = "west"
		return s.checkoutRegion
	}
	raw, _ := ReadBody(resp)
	region := parseCheckoutRegion(string(raw))
	if resp.StatusCode >= 400 {
		region = "west"
	}
	s.checkoutRegion = region
	return region
}

func parseCheckoutRegion(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "west"
	}
	var parsed struct {
		Region   string `json:"region"`
		Response string `json:"response"`
	}
	if json.Unmarshal([]byte(raw), &parsed) == nil && parsed.Region != "" {
		return parsed.Region
	}
	if raw == "east" || raw == "west" {
		return raw
	}
	return "west"
}

func (s *Session) attachCheckoutCookies(req *http.Request) {
	if req.URL == nil {
		return
	}
	u := mustURL(WWW)
	checkout := mustURL(CheckoutSDK)
	seen := map[string]bool{}
	var parts []string
	add := func(list []*http.Cookie) {
		for _, ck := range list {
			if ck.Name == "" || seen[ck.Name] || strings.HasPrefix(ck.Name, "KP_UIDz") {
				continue
			}
			seen[ck.Name] = true
			parts = append(parts, ck.Name+"="+ck.Value)
		}
	}
	add(s.client.GetCookies(req.URL))
	add(s.client.GetCookies(checkout))
	add(s.client.GetCookies(u))
	if len(parts) > 0 {
		req.Header.Set("cookie", strings.Join(parts, "; "))
	}
}

func CheckoutPageURL(reserveID, eventURL string) string {
	q := url.Values{
		"ccp_src":           {"3"},
		"ccp_channel":       {"0"},
		"edp":               {eventURL},
		"f_appview":         {"false"},
		"f_appview_ln":      {"false"},
		"f_appview_version": {"1"},
		"f_layout":          {""},
	}
	return CheckoutSDK + "/" + reserveID + "?" + q.Encode()
}

func (s *Session) WarmCheckoutPage(checkoutURL, eventURL string) error {
	req, err := http.NewRequest(http.MethodGet, checkoutURL, nil)
	if err != nil {
		return err
	}
	referer := eventURL
	if referer == "" {
		referer = WWW + "/"
	}
	h := s.WWWHeaders(referer)
	h.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	h.Set("sec-fetch-dest", "document")
	h.Set("sec-fetch-mode", "navigate")
	h.Set("sec-fetch-site", "same-site")
	h.Set("upgrade-insecure-requests", "1")
	s.applyKasadaHeaders(h)
	req.Header = h
	s.attachCheckoutCookies(req)

	HumanDelay(800, 1800)
	resp, err := s.DoMain(req)
	if err != nil {
		return err
	}
	raw, err := ReadBody(resp)
	if err != nil {
		return err
	}
	html := string(raw)
	if err := CheckCheckoutExpired(resp.StatusCode, html); err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("checkout page HTTP %d", resp.StatusCode)
	}
	s.GraphQLReferer = checkoutURL
	if sessionID := parseCheckoutSessionIDRaw(html); sessionID != "" {
		s.GCOSessionID = sessionID
		log.Printf("Checkout sessionId=%s", sessionID)
	} else if sid := checkoutSessionFromCookies(s); sid != "" {
		s.GCOSessionID = sid
		log.Printf("Checkout sessionId=%s (cookie)", sid)
	}
	HumanBrowse()
	return nil
}

func checkoutSessionFromCookies(s *Session) string {
	for _, name := range []string{"SID", "eps_sid"} {
		if sid := s.CookieValue(name); isUsableCheckoutSessionID(sid) {
			return sid
		}
	}
	return ""
}

func (s *Session) resolveCheckoutSession(reserveID, eventURL string) (string, error) {
	if sid := s.PurchaseSessionID(); sid != "" {
		return sid, nil
	}
	if sid := checkoutSessionFromCookies(s); sid != "" {
		s.GCOSessionID = sid
		return sid, nil
	}
	urls := []string{CheckoutSDK + "/" + reserveID + "?"}
	if eventURL != "" {
		urls = append([]string{CheckoutPageURL(reserveID, eventURL)}, urls...)
	}
	var lastFound string
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			HumanDelay(1200, 2500)
		}
		for _, pageURL := range urls {
			html, err := s.fetchCheckoutHTML(pageURL)
			if err != nil {
				continue
			}
			s.GraphQLReferer = pageURL
			if sessionID := parseCheckoutSessionIDRaw(html); isUsableCheckoutSessionID(sessionID) {
				s.GCOSessionID = sessionID
				return sessionID, nil
			}
			if sid := parseCheckoutSessionID(html); sid != "" {
				lastFound = sid
			}
		}
	}
	if lastFound != "" {
		return "", fmt.Errorf("checkout page missing valid sessionId (got %q)", lastFound)
	}
	return "", fmt.Errorf("checkout page missing valid sessionId (reserve=%s)", reserveID)
}

func (s *Session) fetchCheckoutHTML(pageURL string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, pageURL, nil)
	if err != nil {
		return "", err
	}
	h := s.WWWHeaders(WWW + "/")
	h.Set("accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	h.Set("sec-fetch-dest", "document")
	h.Set("sec-fetch-mode", "navigate")
	h.Set("sec-fetch-site", "same-site")
	h.Set("upgrade-insecure-requests", "1")
	req.Header = h
	s.attachCheckoutCookies(req)
	HumanDelay(800, 1500)
	resp, err := s.DoMain(req)
	if err != nil {
		return "", err
	}
	raw, err := ReadBody(resp)
	if err != nil {
		return "", err
	}
	html := string(raw)
	if err := CheckCheckoutExpired(resp.StatusCode, html); err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("checkout page HTTP %d", resp.StatusCode)
	}
	return html, nil
}

func parseCheckoutSessionID(html string) string {
	return parseCheckoutSessionIDRaw(html)
}
