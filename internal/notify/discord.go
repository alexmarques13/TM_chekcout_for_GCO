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

type CartInfo struct {
	Approved bool
	Who      string
	Link     string
}

const webhookUsername = "Cart Watch"

func SendCart(successURL, failURL string, info CartInfo) {
	if info.Approved {
		if err := postEmbed(successURL, approvedEmbed(info)); err != nil {
			log.Printf("discord success webhook: %v", err)
		}
		return
	}
	if err := postEmbed(failURL, deniedEmbed(info)); err != nil {
		log.Printf("discord fail webhook: %v", err)
	}
}

func approvedEmbed(info CartInfo) map[string]any {
	return map[string]any{
		"title": "Cart approved",
		"color": colorSuccess,
		"fields": []map[string]any{
			field("By", dash(info.Who)),
			field("Link", dash(info.Link)),
		},
	}
}

func deniedEmbed(info CartInfo) map[string]any {
	return map[string]any{
		"title": "Cart denied",
		"color": colorFail,
		"fields": []map[string]any{
			field("By", dash(info.Who)),
			field("Link", dash(info.Link)),
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
