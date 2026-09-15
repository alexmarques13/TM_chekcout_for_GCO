package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"

	"tm_checkout/internal/color"
	"tm_checkout/internal/config"
	"tm_checkout/internal/discordwatch"
	"tm_checkout/internal/notify"
	"tm_checkout/internal/summary"

	"github.com/bwmarrin/discordgo"
)

func main() {
	color.Init()
	log.SetOutput(io.Discard)
	log.SetFlags(0)

	lock, err := net.Listen("tcp", "127.0.0.1:47821")
	if err != nil {
		fmt.Println(color.Paint(color.Red, "tm_checkout is already running — close the other window first"))
		os.Exit(1)
	}
	defer lock.Close()

	cfg := config.Load()
	if strings.TrimSpace(cfg.DiscordBotToken) == "" {
		fmt.Println(color.Paint(color.Red, "DISCORD_BOT_TOKEN is empty — set the bot token in .env"))
		os.Exit(1)
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		os.Exit(0)
	}()

	info, err := discordwatch.Resolve(cfg.DiscordLinksWebhook)
	if err != nil {
		fmt.Println(color.Paint(color.Red, err.Error()))
		os.Exit(1)
	}
	name := strings.TrimSpace(info.Name)
	if name == "" {
		name = "webhook"
	}

	var (
		msgN atomic.Int32
		seq  atomic.Int32
	)

	discordwatch.BindCartButtons(func(cart discordwatch.Cart, who string) {
		n := int(seq.Add(1))
		fmt.Println(color.Paint(color.Cyan, summary.Approved(n, who, cart.Link)))
		notify.SendCart(cfg.DiscordSuccessWebhook, cfg.DiscordFailWebhook, notify.CartInfo{
			Approved: true,
			Who:      who,
			Link:     cart.Link,
		})
	}, func(cart discordwatch.Cart, who string) {
		fmt.Println(color.Paint(color.Red, summary.Denied(who, cart.Link)))
		notify.SendCart(cfg.DiscordSuccessWebhook, cfg.DiscordFailWebhook, notify.CartInfo{
			Approved: false,
			Who:      who,
			Link:     cart.Link,
		})
	})

	loaded, err := discordwatch.InitPending("pending_carts.json")
	if err != nil {
		fmt.Println(color.Paint(color.Red, "pending carts: "+err.Error()))
		os.Exit(1)
	}

	session, _, err := discordwatch.Start(cfg.DiscordLinksWebhook, cfg.DiscordBotToken, func(sess *discordgo.Session, ev discordwatch.Event) {
		if !discordwatch.MatchesWatchBot(ev, cfg.DiscordWatchBot) {
			return
		}
		if !discordwatch.LooksLikeCart(ev.Text) {
			return
		}
		link := discordwatch.ExtractGetLink(ev.Text)
		if link == "" {
			fmt.Println(color.Paint(color.Red, "cart skipped — no getLink"))
			return
		}
		if err := discordwatch.ReplyCart(sess, ev, link); err != nil {
			fmt.Println(color.Paint(color.Red, "reply buttons: "+err.Error()))
			return
		}
		n := int(msgN.Add(1))
		fmt.Println(color.Paint(color.Cyan, summary.CartPending(n, link)))
	})
	if err != nil {
		fmt.Println(color.Paint(color.Red, err.Error()))
		os.Exit(1)
	}
	defer session.Close()

	fmt.Println(color.Paint(color.Cyan, summary.Startup(name, info.ChannelID, cfg.DiscordWatchBot)))
	if loaded > 0 {
		fmt.Println(color.Paint(color.Cyan, summary.LoadedPending(loaded)))
	}
	select {}
}
