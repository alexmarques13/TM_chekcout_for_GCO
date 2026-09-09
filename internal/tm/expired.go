package tm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

var ErrCheckoutExpired = errors.New("checkout page expired")

var checkoutExpiredTextMarkers = []string{
	"sorry... time's up",
	"sorry, time's up",
	"time's up",
	"your session has expired",
	"your session expired",
	"session has expired",
	"session expired",
	"reservation has expired",
	"reservation expired",
	"reserve expired",
	"reservation is expired",
	"no longer being held",
	"tickets are no longer",
	"tickets have been released",
	"could not reserve your tickets",
	"we could not reserve",
	"reservation is no longer valid",
	"checkout session expired",
	"this checkout link has expired",
	"link has expired",
}

var checkoutExpiredJSONMarkers = []string{
	"RESERVE_EXPIRED",
	"SESSION_EXPIRED",
	"RESERVATION_EXPIRED",
	"CHECKOUT_EXPIRED",
}

var checkoutExpiredStatusKeys = []string{
	"sessionStatus",
	"reservationStatus",
	"reserveStatus",
	"checkoutStatus",
	"status",
}

func IsCheckoutExpired(err error) bool {
	return errors.Is(err, ErrCheckoutExpired)
}

func CheckCheckoutExpired(statusCode int, html string) error {
	if statusCode == 404 || statusCode == 410 {
		return fmt.Errorf("%w (HTTP %d)", ErrCheckoutExpired, statusCode)
	}

	lower := strings.ToLower(html)
	for _, marker := range checkoutExpiredTextMarkers {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("%w (%s)", ErrCheckoutExpired, marker)
		}
	}

	if m := checkoutNextDataRE.FindStringSubmatch(html); len(m) > 1 {
		var next map[string]any
		if json.Unmarshal([]byte(m[1]), &next) == nil {
			if reason := expiredReasonFromJSON(next); reason != "" {
				return fmt.Errorf("%w (%s)", ErrCheckoutExpired, reason)
			}
		}
	}

	return nil
}

func expiredReasonFromJSON(root map[string]any) string {
	var found string
	walkJSON(root, func(key string, val any) {
		if found != "" {
			return
		}
		for _, statusKey := range checkoutExpiredStatusKeys {
			if !strings.EqualFold(key, statusKey) {
				continue
			}
			if s, ok := val.(string); ok && isExpiredStatusValue(s) {
				found = key + "=" + s
				return
			}
		}
		upperKey := strings.ToUpper(key)
		if strings.Contains(upperKey, "ERROR") || strings.Contains(upperKey, "MESSAGE") {
			if s, ok := val.(string); ok && containsExpiredMarker(s) {
				found = s
			}
		}
	})
	return found
}

func isExpiredStatusValue(v string) bool {
	upper := strings.ToUpper(strings.TrimSpace(v))
	if upper == "" {
		return false
	}
	for _, marker := range checkoutExpiredJSONMarkers {
		if strings.Contains(upper, marker) {
			return true
		}
	}
	return strings.Contains(upper, "EXPIRED")
}

func containsExpiredMarker(s string) bool {
	lower := strings.ToLower(s)
	for _, marker := range checkoutExpiredTextMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return isExpiredStatusValue(s)
}

func GraphQLErrorExpired(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToUpper(err.Error())
	for _, marker := range checkoutExpiredJSONMarkers {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	if strings.Contains(msg, "EXPIRED") {
		return true
	}
	for _, code := range []string{"U002", "U003", "U004"} {
		if strings.Contains(msg, code) && (strings.Contains(msg, "RESERVE") || strings.Contains(msg, "SESSION")) {
			return true
		}
	}
	return false
}

func WrapGraphQLExpired(err error) error {
	if err == nil {
		return nil
	}
	if GraphQLErrorExpired(err) {
		return fmt.Errorf("%w: %v", ErrCheckoutExpired, err)
	}
	return err
}
