package api

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

var emailRE = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

func EmailsFromCookies(cookies []Cookie) (account, profile string) {
	var found []string
	seen := map[string]bool{}
	add := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		for _, m := range emailRE.FindAllString(raw, -1) {
			key := strings.ToLower(m)
			if seen[key] {
				continue
			}
			seen[key] = true
			found = append(found, m)
		}
	}

	for _, c := range cookies {
		add(c.Value)
		if decoded, err := url.QueryUnescape(c.Value); err == nil {
			add(decoded)
		}
		add(jwtPayload(c.Value))
	}

	if len(found) > 0 {
		account = found[0]
	}
	if len(found) > 1 {
		profile = found[1]
	} else {
		profile = account
	}
	return account, profile
}

func jwtPayload(raw string) string {
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return ""
	}
	payload := parts[1]
	if b, err := base64.RawURLEncoding.DecodeString(payload); err == nil {
		return extractEmailClaims(b)
	}
	if b, err := base64.StdEncoding.DecodeString(padB64(payload)); err == nil {
		return extractEmailClaims(b)
	}
	return ""
}

func extractEmailClaims(payload []byte) string {
	var m map[string]any
	if json.Unmarshal(payload, &m) != nil {
		return string(payload)
	}
	for _, key := range []string{"email", "userEmail", "preferred_username", "upn", "unique_name"} {
		if v, ok := m[key].(string); ok && strings.Contains(v, "@") {
			return v
		}
	}
	return string(payload)
}

func padB64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}
