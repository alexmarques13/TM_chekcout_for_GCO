package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

const (
	colorSuccess = 0x2ecc71
	colorFail    = 0xe74c3c
)

type CheckoutInfo struct {
	Approved  bool
	Success   bool
	Store     string
	EventID   string
	Account   string
	Profile   string
	CardLast4 string
	OrderID   string
	Error     string
}

const webhookUsername = "TM Checkout"

func SendCheckout(successURL, failURL string, info CheckoutInfo) {
	if !info.Approved {
		return
	}
	if info.Success {
		if err := postEmbed(successURL, successEmbed(info)); err != nil {
			log.Printf("discord success webhook: %v", err)
		}
		return
	}
	if err := postEmbed(failURL, failEmbed(info)); err != nil {
		log.Printf("discord fail webhook: %v", err)
	}
}

func successEmbed(info CheckoutInfo) map[string]any {
	return map[string]any{
		"title": "Successful checkout",
		"color": colorSuccess,
		"footer": map[string]any{"text": "after Approve"},
		"fields": []map[string]any{
			field("Store", dash(info.Store, "US")),
			field("Event ID", dash(info.EventID)),
			field("Account", dash(info.Account)),
			field("Profile", dash(firstNonEmpty(info.Profile, info.Account))),
			field("Card", dash(info.CardLast4)),
			field("Order", dash(info.OrderID)),
		},
	}
}

func failEmbed(info CheckoutInfo) map[string]any {
	errText := strings.TrimSpace(info.Error)
	if errText == "" {
		errText = "unknown error"
	}
	return map[string]any{
		"title":       "Payment failed",
		"color":       colorFail,
		"description": truncate("Checkout failed: "+errText, 4000),
		"footer":      map[string]any{"text": "after Approve"},
		"fields": []map[string]any{
			field("Store", dash(info.Store, "US")),
			field("Event ID", dash(info.EventID)),
			field("Account", dash(info.Account)),
			field("Profile", dash(firstNonEmpty(info.Profile, info.Account))),
			field("Card", dash(info.CardLast4)),
		},
	}
}

func postEmbed(webhookURL string, embed map[string]any) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return nil
	}
	body, err := json.Marshal(map[string]any{
		"username": webhookUsername,
		"embeds":   []any{embed},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	log.Printf("← Discord webhook %d", resp.StatusCode)
	return nil
}

func field(name, value string) map[string]any {
	return map[string]any{"name": name, "value": value, "inline": true}
}

func dash(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return "—"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
