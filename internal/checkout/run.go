package checkout

import (
	"fmt"
	"log"
	"strings"

	"tm_checkout/internal/api"
	"tm_checkout/internal/captcha"
	"tm_checkout/internal/cards"
	"tm_checkout/internal/config"
	"tm_checkout/internal/notify"
	"tm_checkout/internal/tm"
)

type Result struct {
	Success   bool
	Expired   bool
	Store     string
	EventID   string
	Account   string
	Profile   string
	OrderID   string
	Status    string
	RequestID string
	CardLast4 string
	Error     string
	SpentUSD  float64
}

func Run(linkURL, cardsPath string, cardIndex int) (Result, error) {
	res, err := run(linkURL, cardsPath, cardIndex)
	if err != nil && res.Error == "" {
		res.Error = err.Error()
	}
	if res.Store == "" {
		res.Store = "US"
	}
	if err != nil && strings.Contains(res.Error, "KagedCapKey is empty") {
		return res, err
	}
	cfg := config.Load()
	notify.SendCheckout(cfg.DiscordSuccessWebhook, cfg.DiscordFailWebhook, notify.CheckoutInfo{
		Success:   res.Success,
		Store:     res.Store,
		EventID:   res.EventID,
		Account:   res.Account,
		Profile:   res.Profile,
		CardLast4: res.CardLast4,
		OrderID:   res.OrderID,
		Error:     res.Error,
	})
	return res, err
}

func run(linkURL, cardsPath string, cardIndex int) (Result, error) {
	res := Result{Store: "US"}
	cfg := config.Load()
	if cfg.KagedCapKey == "" {
		return res, fmt.Errorf("KagedCapKey is empty — set it in .env")
	}

	log.Printf("→ GET getLink")
	session, err := api.FetchSession(linkURL)
	if err != nil {
		return res, fmt.Errorf("getLink: %w", err)
	}
	log.Printf("← checkout url ready | cookies=%d | proxy=%s", len(session.Cookies), maskProxy(session.Proxy))
	res.Account, res.Profile = api.EmailsFromCookies(session.Cookies)
	if _, eventURL, err := tm.ParseCheckoutURL(session.CheckoutURL); err == nil {
		res.EventID = extractEventFromURL(eventURL)
	}

	allCards, err := cards.LoadAll(cardsPath)
	if err != nil {
		return res, err
	}
	if cardIndex < 0 || cardIndex >= len(allCards) {
		return res, fmt.Errorf("card index %d out of range (have %d cards)", cardIndex, len(allCards))
	}
	card := allCards[cardIndex]
	res.CardLast4 = card.Last4()
	if card.Account != "" {
		res.Account = card.Account
	}
	if card.Email != "" {
		res.Profile = card.Email
	}
	log.Printf("• card from CSV: ****%s (%s)", card.Last4(), card.Name)

	tmSess, err := tm.NewSession(session.Proxy, session.Cookies)
	if err != nil {
		return res, fmt.Errorf("session: %w", err)
	}
	defer tmSess.Close()

	checkoutURL := session.CheckoutURL
	reserveID, eventURL, err := tm.ParseCheckoutURL(checkoutURL)
	if err != nil {
		return res, err
	}
	log.Printf("• reserveId=%s", reserveID)

	log.Printf("→ GET checkout page")
	if err := tmSess.WarmCheckoutPage(checkoutURL, eventURL); err != nil {
		if tm.IsCheckoutExpired(err) {
			res.Expired = true
			res.Error = err.Error()
			return res, err
		}
		return res, fmt.Errorf("warm checkout: %w", err)
	}

	if acc, prof := api.EmailsFromCookies(tmSess.IdentityCookies()); acc != "" {
		if res.Account == "" {
			res.Account = acc
		}
		if res.Profile == "" {
			res.Profile = prof
		}
	}

	cart, err := tmSess.ParseCheckoutPage(checkoutURL)
	if err != nil {
		if tm.IsCheckoutExpired(err) {
			res.Expired = true
			res.Error = err.Error()
			return res, err
		}
		log.Printf("cart parse warning: %v", err)
		cart = &tm.CheckoutCart{
			ReserveID:     reserveID,
			EventURL:      eventURL,
			SystemEventID: tmSess.Config.SystemEventID,
		}
	}
	if cart.SystemEventID == "" {
		cart.SystemEventID = extractEventFromURL(eventURL)
	}
	if cart.SystemEventID != "" {
		res.EventID = cart.SystemEventID
	}
	if cart.Account != "" {
		res.Account = cart.Account
	}
	if cart.Profile != "" {
		res.Profile = cart.Profile
	}
	pendingSpent := 0.0
	if cart.QuotedCents > 0 {
		pendingSpent = float64(cart.QuotedCents) / 100
	} else if cart.Offer.Total() > 0 {
		pendingSpent = float64(cart.Offer.Total()) / 100
	}
	log.Printf("• event=%s section=%s row=%s seat=%s offer=%s",
		cart.SystemEventID, cart.Section, cart.Row, cart.Seat, cart.Offer.OfferID)

	cap := captcha.New(cfg.KagedCapKey).WithProxy(session.Proxy)
	log.Printf("→ POST kagedcap recaptcha")
	recaptchaToken, err := cap.SolveCheckoutRecaptcha(checkoutURL)
	if err != nil {
		return res, fmt.Errorf("recaptcha: %w", err)
	}
	log.Printf("← recaptcha token ready")

	out, err := tmSess.CompleteSDKCheckout(tm.SDKCheckoutInput{
		ReserveID:      cart.ReserveID,
		EventURL:       cart.EventURL,
		SystemEventID:  cart.SystemEventID,
		Section:        cart.Section,
		Row:            cart.Row,
		Seat:           cart.Seat,
		Offer:          cart.Offer,
		Card:           config.CardFromCSV(card),
		Billing:        config.BillingFromCSV(card),
		RecaptchaToken: recaptchaToken,
	})
	if err != nil {
		if tm.IsCheckoutExpired(err) {
			res.Expired = true
		}
		res.Error = err.Error()
		if out != nil {
			res.RequestID = out.RequestID
			res.Status = out.Status
		}
		return res, err
	}
	res.Success = true
	res.SpentUSD = pendingSpent
	res.OrderID = out.OrderID
	res.Status = out.Status
	res.RequestID = out.RequestID
	return res, nil
}

func RunAll(linkURL, cardsPath string) []Result {
	allCards, err := cards.LoadAll(cardsPath)
	if err != nil {
		return []Result{{Error: err.Error()}}
	}
	var results []Result
	for i := range allCards {
		log.Printf("=== card %d/%d ===", i+1, len(allCards))
		res, err := Run(linkURL, cardsPath, i)
		results = append(results, res)
		if err != nil {
			log.Printf("card %d failed: %v", i+1, err)
			if res.Expired {
				log.Printf("checkout expired — stopping")
				break
			}
		} else {
			log.Printf("card %d success: order=%s status=%s", i+1, res.OrderID, res.Status)
		}
		if res.Success {
			break
		}
	}
	return results
}

func maskProxy(raw string) string {
	parts := strings.Split(raw, ":")
	if len(parts) >= 2 {
		return parts[0] + ":" + parts[1]
	}
	return raw
}

func extractEventFromURL(eventURL string) string {
	parts := strings.Split(strings.Trim(eventURL, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasPrefix(parts[i], "0") && len(parts[i]) >= 10 {
			return parts[i]
		}
	}
	return ""
}
