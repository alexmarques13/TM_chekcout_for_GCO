package discordwatch

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/bwmarrin/discordgo"
)

const (
	cartApprovePrefix = "cart_ok:"
	cartDenyPrefix    = "cart_no:"
)

var getLinkRE = regexp.MustCompile(`(?i)https?://(?:www\.)?encsoft\.app/api/bot/getLink\?id=[A-Za-z0-9_-]+`)

type Cart struct {
	ID   string `json:"id"`
	Link string `json:"link"`
}

var (
	pendingCarts sync.Map
	approveFn    atomic.Value
	denyFn       atomic.Value
)

func ExtractGetLink(text string) string {
	m := getLinkRE.FindString(text)
	return strings.TrimRight(strings.TrimSpace(m), ".,);]>")
}

func LooksLikeCart(text string) bool {
	return ExtractGetLink(text) != ""
}

func ReplyCart(s *discordgo.Session, ev Event, link string) error {
	if s == nil {
		return fmt.Errorf("discord session is nil")
	}
	id := strings.TrimSpace(ev.ID)
	if id == "" {
		return fmt.Errorf("missing source message id")
	}
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Errorf("missing getLink")
	}
	_, err := s.ChannelMessageSendComplex(ev.ChannelID, &discordgo.MessageSend{
		Content: "Cart detected. Approve to checkout or Deny to cancel.",
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "Approve",
						Style:    discordgo.SuccessButton,
						CustomID: cartApprovePrefix + id,
					},
					discordgo.Button{
						Label:    "Deny",
						Style:    discordgo.DangerButton,
						CustomID: cartDenyPrefix + id,
					},
				},
			},
		},
		Reference: &discordgo.MessageReference{
			MessageID: ev.ID,
			ChannelID: ev.ChannelID,
		},
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
	if err != nil {
		return err
	}
	return storePending(Cart{ID: id, Link: link})
}

func BindCartButtons(onApprove, onDeny func(Cart, string)) {
	approveFn.Store(onApprove)
	denyFn.Store(onDeny)
}

func registerCartButtons(s *discordgo.Session) {
	if s == nil {
		return
	}
	s.AddHandler(func(sess *discordgo.Session, i *discordgo.InteractionCreate) {
		handleCartButton(sess, i)
	})
}

func handleCartButton(sess *discordgo.Session, i *discordgo.InteractionCreate) {
	if sess == nil || i == nil || i.Type != discordgo.InteractionMessageComponent {
		return
	}
	data := i.MessageComponentData()
	if data.ComponentType != 0 && data.ComponentType != discordgo.ButtonComponent {
		return
	}
	action, cartID := parseCartButton(data.CustomID)
	if action == "" || cartID == "" {
		return
	}
	if sess.State != nil && sess.State.User != nil {
		if id := clickerID(i); id != "" && id == sess.State.User.ID {
			return
		}
	}
	cart, ok := takePending(cartID)
	if !ok {
		_ = sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Already approved or denied.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}
	who := clickerName(i)
	switch action {
	case "approve":
		_ = sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Approved by **" + who + "** — checkout started.",
				Components: []discordgo.MessageComponent{},
			},
		})
		if fn, _ := approveFn.Load().(func(Cart, string)); fn != nil {
			go fn(cart, who)
		}
	case "deny":
		_ = sess.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content:    "Denied by **" + who + "** — canceled. Checkout was not started.",
				Components: []discordgo.MessageComponent{},
			},
		})
		if fn, _ := denyFn.Load().(func(Cart, string)); fn != nil {
			go fn(cart, who)
		}
	}
}

func parseCartButton(customID string) (action, cartID string) {
	switch {
	case strings.HasPrefix(customID, cartApprovePrefix):
		return "approve", strings.TrimPrefix(customID, cartApprovePrefix)
	case strings.HasPrefix(customID, cartDenyPrefix):
		return "deny", strings.TrimPrefix(customID, cartDenyPrefix)
	default:
		return "", ""
	}
}

func clickerID(i *discordgo.InteractionCreate) string {
	if i == nil {
		return ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID
	}
	if i.User != nil {
		return i.User.ID
	}
	return ""
}

func clickerName(i *discordgo.InteractionCreate) string {
	if i == nil {
		return "user"
	}
	if i.Member != nil && i.Member.User != nil {
		if n := strings.TrimSpace(i.Member.User.Username); n != "" {
			return n
		}
	}
	if i.User != nil {
		if n := strings.TrimSpace(i.User.Username); n != "" {
			return n
		}
	}
	return "user"
}
