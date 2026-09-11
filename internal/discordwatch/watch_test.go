package discordwatch

import (
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestLooksLikeCartAndGetLink(t *testing.T) {
	text := "Successfully Carted!\nhttps://checkout.ticketmaster.com/34b366d60cda49acad00f512f17b7edd?\nhttps://encsoft.app/api/bot/getLink?id=EC4B136B4D7A"
	if !LooksLikeCart(text) {
		t.Fatal("cart not detected")
	}
	link := ExtractGetLink(text)
	if link != "https://encsoft.app/api/bot/getLink?id=EC4B136B4D7A" {
		t.Fatalf("link=%q", link)
	}
	if LooksLikeCart("hello world") {
		t.Fatal("plain text is not a cart")
	}
	if LooksLikeCart("Successfully Carted!\nhttps://checkout.ticketmaster.com/abc") {
		t.Fatal("ticketmaster url is not a cart")
	}
}

func TestParseCartButton(t *testing.T) {
	action, id := parseCartButton("cart_ok:123")
	if action != "approve" || id != "123" {
		t.Fatalf("%s %s", action, id)
	}
	action, id = parseCartButton("cart_no:123")
	if action != "deny" || id != "123" {
		t.Fatalf("%s %s", action, id)
	}
	action, id = parseCartButton("other")
	if action != "" || id != "" {
		t.Fatalf("%s %s", action, id)
	}
}

func TestMatchesWatchBot(t *testing.T) {
	if !MatchesWatchBot(Event{Author: "Encore IO"}, "Encore IO") {
		t.Fatal("exact match")
	}
	if !MatchesWatchBot(Event{Author: "encore io"}, "Encore IO") {
		t.Fatal("case-insensitive")
	}
	if MatchesWatchBot(Event{Author: "fernando20208020"}, "Encore IO") {
		t.Fatal("other user should not match")
	}
	if MatchesWatchBot(Event{Author: "degensora"}, "Encore IO") {
		t.Fatal("other user should not match")
	}
}

func TestAuthorName(t *testing.T) {
	if got := AuthorName(&discordgo.Message{Author: &discordgo.User{Username: "alice"}}); got != "alice" {
		t.Fatalf("got %q", got)
	}
	if got := AuthorName(&discordgo.Message{WebhookID: "1"}); got != "webhook" {
		t.Fatalf("got %q", got)
	}
}

func TestMessageTextIncludesEmbeds(t *testing.T) {
	got := MessageText(&discordgo.Message{
		Content: "hello",
		Embeds: []*discordgo.MessageEmbed{{
			Title:       "drop",
			Description: "https://encsoft.app/api/bot/getLink?id=ABC123",
			URL:         "https://example.com",
			Fields: []*discordgo.MessageEmbedField{{
				Name:  "link",
				Value: "https://encsoft.app/api/bot/getLink?id=DEF456",
			}},
		}},
		Attachments: []*discordgo.MessageAttachment{{
			URL: "https://cdn.discordapp.com/file.txt",
		}},
	})
	for _, want := range []string{
		"hello",
		"getLink?id=ABC123",
		"getLink?id=DEF456",
		"cdn.discordapp.com/file.txt",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}
