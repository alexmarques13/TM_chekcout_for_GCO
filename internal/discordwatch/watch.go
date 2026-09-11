package discordwatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Info struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ChannelID     string `json:"channel_id"`
	GuildID       string `json:"guild_id"`
	ContentIntent bool   `json:"-"`
}

type Event struct {
	ID        string
	ChannelID string
	Author    string
	Webhook   bool
	Text      string
}

func Resolve(webhookURL string) (Info, error) {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return Info{}, fmt.Errorf("DISCORD_LINKS_WEBHOOK is empty — set it in .env")
	}

	req, err := http.NewRequest(http.MethodGet, webhookURL, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("User-Agent", "TMCheckout/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, fmt.Errorf("resolve webhook: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode >= 300 {
		return Info{}, fmt.Errorf("resolve webhook: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var info Info
	if err := json.Unmarshal(raw, &info); err != nil {
		return Info{}, fmt.Errorf("parse webhook: %w", err)
	}
	if strings.TrimSpace(info.ChannelID) == "" {
		return Info{}, fmt.Errorf("webhook has no channel_id")
	}
	return info, nil
}

func Listen(ctx context.Context, webhookURL, botToken string, onEvent func(Event)) (Info, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	session, info, err := Start(webhookURL, botToken, func(_ *discordgo.Session, ev Event) {
		if onEvent != nil {
			onEvent(ev)
		}
	})
	if err != nil {
		return info, err
	}
	defer session.Close()
	<-ctx.Done()
	if ctx.Err() == context.Canceled {
		return info, nil
	}
	return info, ctx.Err()
}

func Start(webhookURL, botToken string, onEvent func(*discordgo.Session, Event)) (*discordgo.Session, Info, error) {
	if onEvent == nil {
		return nil, Info{}, fmt.Errorf("missing message handler")
	}
	info, err := Resolve(webhookURL)
	if err != nil {
		return nil, Info{}, err
	}
	botToken = strings.TrimSpace(botToken)
	if botToken == "" {
		return nil, info, fmt.Errorf("DISCORD_BOT_TOKEN is empty — set the bot token in .env")
	}

	bind := func(s *discordgo.Session) {
		s.AddHandler(func(sess *discordgo.Session, m *discordgo.MessageCreate) {
			if m == nil || m.Message == nil || m.ChannelID != info.ChannelID {
				return
			}
			if sess.State != nil && sess.State.User != nil && m.Author != nil && m.Author.ID == sess.State.User.ID {
				return
			}
			full := enrich(webhookURL, fetchMessage(sess, m.Message))
			if full.ChannelID == "" {
				full.ChannelID = m.ChannelID
			}
			onEvent(sess, Event{
				ID:        full.ID,
				ChannelID: full.ChannelID,
				Author:    AuthorName(full),
				Webhook:   strings.TrimSpace(full.WebhookID) != "",
				Text:      MessageText(full),
			})
		})
		registerCartButtons(s)
	}

	privileged := discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	session, err := openSession(botToken, privileged, bind)
	if err != nil {
		return nil, info, fmt.Errorf("discord gateway: %w", err)
	}
	info.ContentIntent = true
	return session, info, nil
}

func openSession(botToken string, intents discordgo.Intent, bind func(*discordgo.Session)) (*discordgo.Session, error) {
	session, err := discordgo.New("Bot " + botToken)
	if err != nil {
		return nil, fmt.Errorf("discord session: %w", err)
	}
	session.Identify.Intents = intents
	bind(session)
	if err := session.Open(); err != nil {
		_ = session.Close()
		return nil, err
	}
	return session, nil
}

func fetchMessage(s *discordgo.Session, m *discordgo.Message) *discordgo.Message {
	if s == nil || m == nil {
		return m
	}
	if messageHasBody(m) {
		return m
	}
	fetched, err := s.ChannelMessage(m.ChannelID, m.ID)
	if err != nil || fetched == nil {
		return m
	}
	return fetched
}

func enrich(webhookURL string, m *discordgo.Message) *discordgo.Message {
	if messageHasBody(m) {
		return m
	}
	fetched, err := FetchWebhookMessage(webhookURL, m.ID)
	if err != nil || fetched == nil {
		return m
	}
	if fetched.ChannelID == "" {
		fetched.ChannelID = m.ChannelID
	}
	return fetched
}

func messageHasBody(m *discordgo.Message) bool {
	if m == nil {
		return false
	}
	if strings.TrimSpace(m.Content) != "" {
		return true
	}
	if len(m.Embeds) > 0 || len(m.Attachments) > 0 {
		return true
	}
	return false
}

func FetchWebhookMessage(webhookURL, messageID string) (*discordgo.Message, error) {
	root := webhookAPIRoot(webhookURL)
	messageID = strings.TrimSpace(messageID)
	if root == "" || messageID == "" {
		return nil, fmt.Errorf("missing webhook or message id")
	}
	u := root + "/messages/" + url.PathEscape(messageID)
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "TMCheckout/1.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var msg discordgo.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

func webhookAPIRoot(webhookURL string) string {
	u := strings.TrimRight(strings.TrimSpace(webhookURL), "/")
	u = strings.Replace(u, "://discordapp.com/", "://discord.com/", 1)
	return u
}

func AuthorName(m *discordgo.Message) string {
	if m == nil {
		return "-"
	}
	if m.Author != nil {
		name := strings.TrimSpace(m.Author.Username)
		if name != "" {
			return name
		}
	}
	if strings.TrimSpace(m.WebhookID) != "" {
		return "webhook"
	}
	return "-"
}

func MatchesWatchBot(ev Event, want string) bool {
	want = normalizeName(want)
	if want == "" {
		return true
	}
	return normalizeName(ev.Author) == want
}

func normalizeName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func MessageText(m *discordgo.Message) string {
	if m == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(m.Content)
	b.WriteByte('\n')
	for _, e := range m.Embeds {
		if e == nil {
			continue
		}
		b.WriteString(e.Title)
		b.WriteByte('\n')
		b.WriteString(e.Description)
		b.WriteByte('\n')
		b.WriteString(e.URL)
		b.WriteByte('\n')
		if e.Author != nil {
			b.WriteString(e.Author.Name)
			b.WriteByte('\n')
			b.WriteString(e.Author.URL)
			b.WriteByte('\n')
		}
		for _, f := range e.Fields {
			if f == nil {
				continue
			}
			b.WriteString(f.Name)
			b.WriteByte('\n')
			b.WriteString(f.Value)
			b.WriteByte('\n')
		}
	}
	for _, a := range m.Attachments {
		b.WriteString(a.URL)
		b.WriteByte('\n')
	}
	if m.ReferencedMessage != nil {
		b.WriteString(MessageText(m.ReferencedMessage))
	}
	return b.String()
}
