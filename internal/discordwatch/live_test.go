package discordwatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func TestWebhookAPIRoot(t *testing.T) {
	got := webhookAPIRoot("https://discordapp.com/api/webhooks/1/token/")
	want := "https://discord.com/api/webhooks/1/token"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestLiveGatewayAndWebhookDetect(t *testing.T) {
	loadDotEnv()
	webhook := strings.TrimSpace(os.Getenv("DISCORD_LINKS_WEBHOOK"))
	token := strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN"))
	if webhook == "" || token == "" {
		t.Skip("DISCORD_LINKS_WEBHOOK or DISCORD_BOT_TOKEN missing")
	}

	marker := "webhook testing"
	got := make(chan Event, 8)

	session, info, err := Start(webhook, token, func(_ *discordgo.Session, ev Event) {
		if strings.Contains(strings.ToLower(ev.Text), marker) {
			select {
			case got <- ev:
			default:
			}
		}
	})
	if err != nil {
		t.Fatalf("gateway: %v", err)
	}
	defer session.Close()
	if info.ChannelID == "" {
		t.Fatal("missing channel id")
	}
	if !info.ContentIntent {
		t.Fatal("message content intent was not enabled")
	}

	msgID, err := postWebhookMessage(webhook, marker)
	if err != nil {
		t.Fatalf("post webhook: %v", err)
	}
	defer func() { _ = deleteWebhookMessage(webhook, msgID) }()
	if sent, sendErr := session.ChannelMessageSend(info.ChannelID, marker); sendErr == nil && sent != nil {
		defer func() { _ = session.ChannelMessageDelete(info.ChannelID, sent.ID) }()
	}

	select {
	case ev := <-got:
		if !strings.Contains(strings.ToLower(ev.Text), marker) {
			t.Fatalf("detected text missing %q: %q", marker, ev.Text)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for webhook testing")
	}
}

func loadDotEnv() {
	candidates := []string{".env", "../.env", "../../.env"}
	if root, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(root, "..", "..", ".env"))
	}
	for _, p := range candidates {
		_ = godotenv.Load(p)
	}
}

func postWebhookMessage(webhookURL, content string) (string, error) {
	u := webhookAPIRoot(webhookURL) + "?wait=true"
	body, err := json.Marshal(map[string]any{"content": content})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TMCheckout/1.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var msg struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &msg); err != nil {
		return "", err
	}
	if msg.ID == "" {
		return "", fmt.Errorf("webhook post missing message id")
	}
	return msg.ID, nil
}

func deleteWebhookMessage(webhookURL, messageID string) error {
	u := webhookAPIRoot(webhookURL) + "/messages/" + messageID
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "TMCheckout/1.0")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 && resp.StatusCode != http.StatusNoContent {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return nil
}
