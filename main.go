package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"

	"tm_checkout/internal/cards"
	"tm_checkout/internal/checkout"
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

	if _, err := cards.LoadAll("cards.csv"); err != nil {
		fmt.Println(color.Paint(color.Red, err.Error()))
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
		msgN     atomic.Int32
		seq      atomic.Int32
		cardNext cards.Rotator
		mu       sync.Mutex
		doneN    int
		okN      int
		failN    int
		spentN   float64
	)

	runApproved := func(cart discordwatch.Cart, who string) {
		n := int(seq.Add(1))
		card, cardIdx, cardTotal, cardErr := cardNext.Next("cards.csv")
		if cardErr != nil {
			fmt.Println(color.Paint(color.Red, "cards.csv: "+cardErr.Error()))
			return
		}
		fmt.Println(color.Paint(color.Cyan, summary.Approved(n, who, cart.Link, cardIdx, cardTotal, card.Last4())))
		res, runErr := checkout.Run(cart.Link, "cards.csv", cardIdx)
		errText := res.Error
		if errText == "" && runErr != nil {
			errText = runErr.Error()
		}
		notify.SendCheckout(cfg.DiscordSuccessWebhook, cfg.DiscordFailWebhook, notify.CheckoutInfo{
			Approved:  true,
			Success:   res.Success,
			Store:     res.Store,
			EventID:   res.EventID,
			Account:   res.Account,
			Profile:   res.Profile,
			CardLast4: res.CardLast4,
			OrderID:   res.OrderID,
			Error:     errText,
		})
		mu.Lock()
		doneN++
		if res.Success {
			okN++
			spentN += res.SpentUSD
		} else {
			failN++
		}
		fmt.Println(formatResult(n, cart.Link, res, doneN, okN, failN, spentN))
		mu.Unlock()
	}

	discordwatch.BindCartButtons(runApproved, func(cart discordwatch.Cart, who string) {
		fmt.Println(color.Paint(color.Red, summary.Denied(who, cart.Link)))
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

func formatResult(index int, raw string, res checkout.Result, done, ok, failed int, totalSpent float64) string {
	line := summary.CheckoutLine(summary.Result{
		Index:      index,
		Link:       raw,
		Success:    res.Success,
		Expired:    res.Expired,
		EventID:    res.EventID,
		OrderID:    res.OrderID,
		CardLast4:  res.CardLast4,
		SpentUSD:   res.SpentUSD,
		Error:      res.Error,
		Done:       done,
		OK:         ok,
		Fail:       failed,
		TotalSpent: totalSpent,
	})
	if res.Success {
		return color.BoldPaint(color.Green, line)
	}
	return color.BoldPaint(color.Red, line)
}
