package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	defaultSuccessWebhook = "https://discordapp.com/api/webhooks/1547050836432130108/f8lONQ28k0GxLimCaejeZG25epuVdZYgrmyk5NFI3HyjkwRrOFIEpasST92sPLn_yY3x"
	defaultFailWebhook    = "https://discordapp.com/api/webhooks/1547050909736112140/tKoa2JOvmctGZs3FE38fYf9kMlhwRE4voHEQxd9M6A0qMS1AUV_Gft5p6u8MkRlU8KNt"
)

type Env struct {
	KagedCapKey           string
	DiscordSuccessWebhook string
	DiscordFailWebhook    string
}

func Load() Env {
	_ = godotenv.Load()
	return Env{
		KagedCapKey: firstNonEmpty(
			os.Getenv("KagedCapKey"),
			os.Getenv("KAGEDCAP_KEY"),
			os.Getenv("KAGEDCAP_API_KEY"),
		),
		DiscordSuccessWebhook: firstNonEmpty(
			os.Getenv("DISCORD_SUCCESS_WEBHOOK"),
			defaultSuccessWebhook,
		),
		DiscordFailWebhook: firstNonEmpty(
			os.Getenv("DISCORD_FAIL_WEBHOOK"),
			defaultFailWebhook,
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
