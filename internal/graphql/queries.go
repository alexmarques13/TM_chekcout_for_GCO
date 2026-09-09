package graphql

const (
	QueryInitPayment = `
query initPayment($jwtInput: JWTInput!) {
  initPayment(jwtInput: $jwtInput) {
    audienceRewardsView {
      accountId
      enabled
    }
    clientToken
    marketId
    marketName
  }
}`

	QueryDelivery = `
query delivery($criteria: Criteria!) {
  loadOptions(criteria: $criteria) {
    deliveryOptions {
      id
      isETicket
      deliveryDelay {
        eventDate
        eTicketDateTime
        eTicketDelay
        offset
        ticketFastDelay
      }
      deliveryMethod
      deliveryTax {
        value
        currencyCode
      }
      description
      expectedDeliveryDate
      name
      price {
        currencyCode
        value
      }
      regions
      subDeliveryOptions {
        id
        description
        name
      }
      type
      variant
    }
  }
}`

	QueryInitUpsell = `
query initUpsell($initUpsellInput: InitUpsellInput!) {
  initUpsell(initUpsellInput: $initUpsellInput) {
    hasRoktParking
    hasRoktUpsells
    sponsorshipId
    doNotSell
  }
}`

	MutationSDKPurchase = `
mutation purchaseMutation($purchaseInput: PurchaseInput!) {
  purchase(purchaseInput: $purchaseInput) {
    requestId
    status
    errors {
      code
      message
    }
  }
}`

	QueryPurchaseStatus = `
query purchaseStatusQuery($getSessionStatusInput: GetSessionStatusInput!) {
  getSessionStatus(getSessionStatusInput: $getSessionStatusInput) {
    errors {
      code
      message
    }
    requestId
    status
    purchaseStatusResponse {
      orderNumber
      errors {
        code
        message
      }
    }
  }
}`
)
