// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package stripe

// Config stores needed information for payment service initialization.
type Config struct {
	StripeSecretKey         string `help:"stripe API secret key" default:""`
	StripePublicKey         string `help:"stripe API public key" default:""`
	StripeFreeTierCouponID  string `help:"stripe free tier coupon ID" default:""`
	StripeWebhookSecret     string `help:"stripe webhookEvents secret token" default:""`
	AutoAdvance             bool   `help:"toggle autoadvance feature for invoice creation" default:"false"`
	ListingLimit            int    `help:"sets the maximum amount of items before we start paging on requests" default:"100" hidden:"true"`
	SkipEmptyInvoices       bool   `help:"if set, skips the creation of empty invoices for customers with zero usage for the billing period" default:"true"`
	MaxParallelCalls        int    `help:"the maximum number of concurrent Stripe API calls in invoicing methods" default:"10"`
	RemoveExpiredCredit     bool   `help:"whether to remove expired package credit or not" default:"true"`
	UseIdempotency          bool   `help:"whether to use idempotency for create/update requests" default:"true"`
	SkuEnabled              bool   `help:"whether we should use SKUs for product usages" default:"false"`
	SkipNoCustomer          bool   `help:"whether to skip the invoicing for users without a Stripe customer. DO NOT SET IN PRODUCTION!" default:"false" hidden:"true"`
	InvItemSKUInDescription bool   `help:"whether to include SKU in the invoice item description" default:"true"`
	MaxCreditCardCount      int    `help:"maximum number of credit cards per customer" default:"8"`
	RoundUpInvoiceUsage     bool   `help:"whether to round up usage quantities on invoices" default:"true"`
	Retries                 RetryConfig
}
