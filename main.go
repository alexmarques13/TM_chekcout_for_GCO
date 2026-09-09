package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"tm_checkout/internal/cards"
	"tm_checkout/internal/checkout"
	"tm_checkout/internal/color"
)

var getLinkRE = regexp.MustCompile(`(?i)https?://[^\s<>"']*getLink\?[^\s<>"']+`)

func main() {
	cardsFlag := flag.String("cards", "cards.csv", "path to cards CSV")
	cardIndex := flag.Int("card", 0, "card index in CSV (0-based)")
	allCards := flag.Bool("all", false, "try all cards until one succeeds")
	flag.Parse()
	color.Init()

	log.SetOutput(io.Discard)
	log.SetFlags(0)

	cardList, err := cards.LoadAll(*cardsFlag)
	if err != nil {
		fmt.Println(color.Paint(color.Red, err.Error()))
		os.Exit(1)
	}
	if *cardIndex < 0 || *cardIndex >= len(cardList) {
		fmt.Println(color.Paint(color.Red, fmt.Sprintf("card index %d out of range (have %d cards)", *cardIndex, len(cardList))))
		os.Exit(1)
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		seq      atomic.Int32
		stopping atomic.Bool
		doneN    int
		okN      int
		failN    int
		spentN   float64
	)

	printPrompt := func() {
		fmt.Print(color.Paint(color.Cyan, "link> "))
	}

	startCheckout := func(raw string) {
		if stopping.Load() {
			return
		}
		n := int(seq.Add(1))
		id := linkID(raw)
		cardIdx := *cardIndex
		if !*allCards {
			cardIdx = (*cardIndex + n - 1) % len(cardList)
		}
		wg.Add(1)
		mu.Lock()
		fmt.Println(color.Paint(color.Cyan, fmt.Sprintf("queued #%d %s card=%s",
			n, id, cardList[cardIdx].Last4())))
		if !stopping.Load() {
			printPrompt()
		}
		mu.Unlock()

		go func(n int, raw string, cardIdx int) {
			defer wg.Done()
			res, runErr := runCheckout(raw, *cardsFlag, cardIdx, *allCards)
			mu.Lock()
			doneN++
			if res.Success {
				okN++
				spentN += res.SpentUSD
			} else {
				failN++
			}
			line := formatLine(n, raw, res, runErr, doneN, okN, failN, spentN)
			fmt.Println()
			fmt.Println(line)
			if !stopping.Load() {
				printPrompt()
			}
			mu.Unlock()
		}(n, raw, cardIdx)
	}

	var finishOnce sync.Once
	finish := func() {
		finishOnce.Do(func() {
			stopping.Store(true)
			wg.Wait()
			os.Exit(0)
		})
	}

	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		os.Exit(0)
	}()

	fmt.Println(color.Paint(color.Cyan, "paste getLink URLs — checkout starts immediately. Type quit to stop accepting links. Ctrl+C kills everything now."))
	printPrompt()

	reader := bufio.NewReader(os.Stdin)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		if stopping.Load() {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			mu.Lock()
			printPrompt()
			mu.Unlock()
			continue
		}
		if isQuit(line) {
			break
		}
		link := extractLink(line)
		if link == "" {
			mu.Lock()
			fmt.Println(color.Paint(color.Red, "not a getLink URL"))
			printPrompt()
			mu.Unlock()
			continue
		}
		startCheckout(link)
	}

	stopping.Store(true)
	finish()
}

func isQuit(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "q", "quit", "exit":
		return true
	default:
		return false
	}
}

func extractLink(raw string) string {
	raw = strings.TrimSpace(raw)
	if m := getLinkRE.FindString(raw); m != "" {
		return strings.TrimRight(m, ".,);]>")
	}
	u, err := url.Parse(raw)
	if err == nil && u.Scheme != "" && strings.Contains(strings.ToLower(u.Path), "getlink") {
		return raw
	}
	return ""
}

func runCheckout(link, cardsPath string, cardIndex int, allCards bool) (checkout.Result, error) {
	if !allCards {
		return checkout.Run(link, cardsPath, cardIndex)
	}
	results := checkout.RunAll(link, cardsPath)
	if len(results) == 0 {
		return checkout.Result{Error: "no cards"}, fmt.Errorf("no cards")
	}
	for _, r := range results {
		if r.Success {
			return r, nil
		}
	}
	last := results[len(results)-1]
	if last.Error != "" {
		return last, fmt.Errorf("%s", last.Error)
	}
	return last, fmt.Errorf("all cards failed")
}

func formatLine(index int, raw string, res checkout.Result, err error, links, ok, failed int, totalSpent float64) string {
	id := linkID(raw)
	totals := fmt.Sprintf("links=%d success=%d failed=%d total=$%.2f", links, ok, failed, totalSpent)
	if res.Success {
		line := fmt.Sprintf("link#%d SUCCESS id=%s event=%s order=%s card=%s spent=$%.2f %s",
			index, id, dash(res.EventID), dash(res.OrderID), dash(res.CardLast4), res.SpentUSD, totals)
		return color.BoldPaint(color.Green, line)
	}
	msg := res.Error
	if msg == "" && err != nil {
		msg = err.Error()
	}
	status := "FAILED"
	if res.Expired {
		status = "EXPIRED"
	}
	line := fmt.Sprintf("link#%d %s id=%s event=%s card=%s spent=$%.2f error=%s %s",
		index, status, id, dash(res.EventID), dash(res.CardLast4), res.SpentUSD, oneLine(msg), totals)
	return color.BoldPaint(color.Red, line)
}

func linkID(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil {
		if id := strings.TrimSpace(u.Query().Get("id")); id != "" {
			return id
		}
	}
	return raw
}

func dash(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "-"
	}
	return s
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return "-"
	}
	if len(s) > 220 {
		return s[:220] + "..."
	}
	return s
}
