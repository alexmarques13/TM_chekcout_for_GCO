package tm

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"tm_checkout/internal/config"
	"tm_checkout/internal/graphql"
)

const CheckoutRecaptchaSiteKey = "6LczUHQrAAAAAGB50LJU2UVRDrXQECmGuyPXUvX9"

type SDKCheckoutInput struct {
	ReserveID      string
	EventURL       string
	SystemEventID  string
	Section        string
	Row            string
	Seat           string
	Offer          SeatOffer
	Card           config.CardConfig
	Billing        config.BillingProfile
	RecaptchaToken string
}

type SDKCheckoutResult struct {
	OrderID   string
	Status    string
	RequestID string
}

func (s *Session) CompleteSDKCheckout(in SDKCheckoutInput) (*SDKCheckoutResult, error) {
	if in.Card.Number == "" || in.Card.CVV == "" {
		return nil, fmt.Errorf("card number and CVV required")
	}
	billing := in.Billing.Normalized()
	if billing.HolderName == "" {
		billing = config.DefaultBilling().Normalized()
	}
	log.Printf("Payment billing: %s | %s %s %s", billing.HolderName, billing.City, billing.State, billing.PostalCode)

	if sid := s.PurchaseSessionID(); !isUsableCheckoutSessionID(sid) {
		s.ResetPurchaseSession()
	} else {
		log.Printf("Checkout sessionId=%s (keeping warmed session)", sid)
	}
	s.GraphQLReferer = CheckoutPageURL(in.ReserveID, in.EventURL)

	HumanCheckoutStep()
	initData, err := s.SDKCheckoutGraphQL("initPayment", graphql.QueryInitPayment, "initPayment",
		"braintree-3ds,caInitPennProvider,caInitRbcProvider,caInitUnitedProvider,caInitWyndhamProvider",
		in.ReserveID, false, map[string]any{
			"jwtInput": map[string]any{
				"additionalTokens":   map[string]bool{"klarna": false},
				"klarnaCustomOffers": nil,
				"requestContext":     map[string]string{"locale": "en-us"},
				"reserveId":          in.ReserveID,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("initPayment: %w", err)
	}
	if s.PurchaseSessionID() == "" {
		if _, err := s.resolvePurchaseSessionID(in.ReserveID, in.EventURL); err != nil {
			log.Printf("Checkout session after initPayment: %v", err)
		}
	}

	clientTokenRaw, _ := nestedAny(initData, "data", "initPayment", "clientToken").(string)
	tokenInfo, err := ParsePaymentClientToken(clientTokenRaw)
	if err != nil {
		return nil, err
	}
	authFingerprint, err := authorizationFingerprint(tokenInfo.BraintreeAuth)
	if err != nil {
		return nil, err
	}

	HumanCheckoutStep()
	deliveryID, err := s.loadDeliveryOption(in)
	if err != nil {
		return nil, err
	}
	log.Printf("Delivery option: %s", deliveryID)

	HumanCheckoutStep()
	upsellData, err := s.SDKCheckoutGraphQL("initUpsell", graphql.QueryInitUpsell, "initUpsell", "rokt",
		in.ReserveID, false, map[string]any{
			"initUpsellInput": map[string]any{
				"requestContext": map[string]string{
					"channel": "desktop.ticketmaster.us",
					"locale":  "en-us",
				},
				"cartItems": []map[string]any{{
					"extTicketType": in.Offer.OfferID,
					"price":         float64(in.Offer.Total()) / 100,
					"quantity":      1,
					"sectionName":   in.Section,
				}},
				"requestId": in.ReserveID,
			},
		})
	if err != nil {
		return nil, fmt.Errorf("initUpsell: %w", err)
	}
	sponsorshipID, _ := nestedAny(upsellData, "data", "initUpsell", "sponsorshipId").(string)

	tokenizer, err := NewBraintreeTokenizer(s.Config.UserAgent, s.Config.ProxyURL)
	if err != nil {
		return nil, err
	}
	defer tokenizer.Close()

	wallet := NewFanWalletClient(s, tokenInfo.Fingerprint, in.ReserveID, s.GraphQLReferer)
	tmNonce, err := s.prepareSDKPaymentNonce(wallet, tokenizer, authFingerprint, in, billing)
	if err != nil {
		return nil, err
	}

	if in.RecaptchaToken == "" {
		return nil, fmt.Errorf("checkout reCAPTCHA token required")
	}

	ticketUSD := float64(in.Offer.Total()) / 100
	if ticketUSD < 0 {
		ticketUSD = 0
	}
	insurance := s.DeclineInsurance(in.ReserveID, in.SystemEventID, ticketUSD, billing.PostalCode)
	log.Printf("Ticket protection: No (optIn=false)")

	expirationTime := time.Now().Add(12 * time.Minute).UTC().Format("2006-01-02T15:04:05.000Z")
	purchaseInput := map[string]any{
		"channel":        "desktop.ticketmaster.us",
		"deliveryId":     deliveryID,
		"deviceType":     "Desktop",
		"domain":         "ticketmaster.com",
		"eventId":        in.SystemEventID,
		"expirationTime": expirationTime,
		"insurance":      insurance,
		"paymentMethods": []map[string]any{{
			"fundingSource": "creditcard",
			"order":         1,
			"paymentToken":  tmNonce,
		}},
		"paymentToken":           tmNonce,
		"recaptchaToken":         in.RecaptchaToken,
		"requestId":              in.ReserveID,
		"sponsorshipId":          sponsorshipID,
		"sponsorshipUpsellPrice": 0,
		"tmUpsells":              []any{},
		"upsellParkWhizId":       nil,
		"upsells":                nil,
		"requestContext": map[string]string{
			"channel": "desktop.ticketmaster.us",
			"locale":  "en-us",
		},
		"tickets": []map[string]any{{
			"section":   in.Section,
			"row":       in.Row,
			"startSeat": in.Seat,
			"endSeat":   in.Seat,
			"ticketTypes": []map[string]any{{
				"id":       in.Offer.OfferID,
				"quantity": 1,
			}},
		}},
	}

	HumanCheckoutStep()
	if !isUsableCheckoutSessionID(s.GCOSessionID) {
		if _, err := s.resolveCheckoutSession(in.ReserveID, in.EventURL); err != nil {
			return nil, fmt.Errorf("load checkout page: %w", err)
		}
	}

	purchaseData, err := s.SDKCheckoutGraphQL("purchaseMutation", graphql.MutationSDKPurchase, "purchase",
		"pate,rokt,fraud-service,braintree-3ds,fvm2", in.ReserveID, true,
		map[string]any{"purchaseInput": purchaseInput})
	if err != nil {
		return nil, fmt.Errorf("purchase: %w", err)
	}
	purchase := nestedMap(purchaseData, "data", "purchase")
	if purchase == nil {
		return nil, fmt.Errorf("purchase missing from response")
	}
	if errs, ok := purchase["errors"].([]any); ok && len(errs) > 0 {
		return nil, fmt.Errorf("%s", graphqlErrJSON(errs))
	}
	status, _ := purchase["status"].(string)
	requestID := strVal(purchase["requestId"])
	if requestID == "" {
		requestID = in.ReserveID
	}
	log.Printf("Purchase submitted: status=%s requestId=%s", status, requestID)

	orderID, finalStatus, err := s.pollSDKPurchaseStatus(requestID)
	if err != nil {
		return &SDKCheckoutResult{RequestID: requestID, Status: status}, err
	}
	return &SDKCheckoutResult{OrderID: orderID, Status: finalStatus, RequestID: requestID}, nil
}

func (s *Session) pollSDKPurchaseStatus(reserveID string) (orderID, status string, err error) {
	deadline := time.Now().Add(90 * time.Second)
	for time.Now().Before(deadline) {
		HumanCheckoutStep()
		data, err := s.SDKCheckoutGraphQL("purchaseStatusQuery", graphql.QueryPurchaseStatus, "getSessionStatus", "",
			reserveID, false, map[string]any{
				"getSessionStatusInput": map[string]string{"requestId": reserveID},
			})
		if err != nil {
			log.Printf("purchase status poll: %v", err)
			continue
		}
		session := nestedMap(data, "data", "getSessionStatus")
		if session == nil {
			continue
		}
		status, _ = session["status"].(string)
		if resp, ok := session["purchaseStatusResponse"].(map[string]any); ok {
			orderID = strVal(resp["orderNumber"])
			if errs, ok := resp["errors"].([]any); ok && len(errs) > 0 {
				return "", status, fmt.Errorf("%s", graphqlErrJSON(errs))
			}
		}
		log.Printf("Purchase status: %s order=%s", status, orderID)
		switch strings.ToUpper(status) {
		case "PURCHASE_SUCCESS", "SUCCESS":
			return orderID, status, nil
		case "PURCHASE_FAILURE", "PURCHASE_FAILED", "FAILED", "ERROR", "RETRY_PURCHASE_FAILURE":
			return "", status, fmt.Errorf("purchase failed: %s", status)
		}
	}
	return orderID, status, fmt.Errorf("purchase status timed out (last=%s)", status)
}

func (s *Session) loadDeliveryOption(in SDKCheckoutInput) (string, error) {
	criteria := map[string]any{
		"eventIds":      []string{in.SystemEventID},
		"inventoryType": "Primary",
		"locale":        "en-us",
		"purchaseType":  "STANDARD",
		"requestId":     in.ReserveID,
		"ticketTypeIds": []string{in.Offer.OfferID},
	}
	if secnames := deliverySecnames(in.Offer); len(secnames) > 0 {
		criteria["secnames"] = secnames
	}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			HumanThink()
		}
		deliveryData, err := s.SDKCheckoutGraphQL("delivery", graphql.QueryDelivery, "loadOptions",
			"venmoTMPlusFlag,venmo3PEFlag", in.ReserveID, false,
			map[string]any{"criteria": criteria})
		if err != nil {
			log.Printf("loadOptions attempt %d: %v", attempt+1, err)
			continue
		}
		if id, err := pickMobileDeliveryID(deliveryData); err == nil {
			return id, nil
		}
	}
	log.Printf("loadOptions failed, using default delivery id 1074")
	return "1074", nil
}

func deliverySecnames(offer SeatOffer) []string {
	level := strings.TrimSpace(offer.PriceLevelID)
	if level == "" {
		return nil
	}
	return []string{"UPPER", "FULL", level}
}

func pickMobileDeliveryID(data map[string]any) (string, error) {
	options, _ := nestedAny(data, "data", "loadOptions", "deliveryOptions").([]any)
	for _, item := range options {
		opt, _ := item.(map[string]any)
		if opt == nil {
			continue
		}
		regions, _ := opt["regions"].([]any)
		for _, region := range regions {
			if region == "USA" {
				if id, _ := opt["id"].(string); id != "" {
					return id, nil
				}
			}
		}
	}
	if len(options) > 0 {
		if opt, ok := options[0].(map[string]any); ok {
			if id, _ := opt["id"].(string); id != "" {
				return id, nil
			}
		}
	}
	return "", fmt.Errorf("no delivery option found")
}

func checkoutAmountDollars(totalCents int) float64 {
	if totalCents <= 0 {
		return 0
	}
	return round2(float64(totalCents) * 1.083 / 100)
}

func (s *Session) prepareSDKPaymentNonce(wallet *FanWalletClient, tokenizer *BraintreeTokenizer, authFingerprint string, in SDKCheckoutInput, billing config.BillingProfile) (string, error) {
	HumanPaymentStep()
	cardToken, err := tokenizer.TokenizeCard(authFingerprint, in.Card)
	if err != nil {
		return "", fmt.Errorf("tokenize card: %w", err)
	}
	cvvToken, err := tokenizer.TokenizeCVV(authFingerprint, in.Card.CVV)
	if err != nil {
		return "", fmt.Errorf("tokenize cvv: %w", err)
	}

	HumanPaymentStep()
	tmNonce, err := wallet.CreateNonce(cardToken, billing)
	if err != nil {
		return "", fmt.Errorf("FanWallet nonce: %w", err)
	}
	log.Printf("FanWallet tm-nonce: %s", tmNonce)

	if err := wallet.AttachCVV(cvvToken, tmNonce); err != nil {
		return "", fmt.Errorf("FanWallet cvv: %w", err)
	}

	providerNonce, bypass, err := wallet.ProviderNonce(tmNonce, in.ReserveID)
	if err != nil {
		return "", fmt.Errorf("FanWallet braintree nonce: %w", err)
	}
	log.Printf("Braintree provider nonce bypass=%v", bypass)
	if bypass {
		return tmNonce, nil
	}

	cardBin := in.Card.Number
	if len(cardBin) > 6 {
		cardBin = cardBin[:6]
	}
	authID, provider3DSNonce, softFail, err := tokenizer.Lookup3DS(authFingerprint, providerNonce, checkoutAmountDollars(in.Offer.Total()), cardBin)
	if err != nil {
		return "", fmt.Errorf("3DS lookup: %w", err)
	}
	if softFail {
		log.Printf("3DS soft-fail; continuing without patch")
		return tmNonce, nil
	}
	if err := wallet.Patch3DS(tmNonce, in.ReserveID, authID, provider3DSNonce); err != nil {
		return "", fmt.Errorf("FanWallet 3DS patch: %w", err)
	}
	return tmNonce, nil
}

func graphqlErrJSON(errs []any) string {
	b, err := json.Marshal(errs)
	if err != nil {
		return fmt.Sprint(errs)
	}
	return string(b)
}
