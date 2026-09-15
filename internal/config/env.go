package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Env struct {
	DiscordSuccessWebhook string
	DiscordFailWebhook    string
	DiscordLinksWebhook   string
	DiscordBotToken       string
	DiscordWatchBot       string
}

func Load() Env {
	_ = godotenv.Load()
	return Env{
		DiscordSuccessWebhook: strings.TrimSpace(os.Getenv("DISCORD_SUCCESS_WEBHOOK")),
		DiscordFailWebhook:    strings.TrimSpace(os.Getenv("DISCORD_FAIL_WEBHOOK")),
		DiscordLinksWebhook: firstNonEmpty(
			os.Getenv("DISCORD_LINKS_WEBHOOK"),
			os.Getenv("DISCORD_WATCH_WEBHOOK"),
		),
		DiscordBotToken: firstNonEmpty(
			os.Getenv("DISCORD_BOT_TOKEN"),
			os.Getenv("DISCORD_TOKEN"),
		),
		DiscordWatchBot: firstNonEmpty(
			os.Getenv("DISCORD_WATCH_BOT"),
			os.Getenv("DISCORD_DETECT_BOT"),
			"Encore IO",
		),
	}
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}
