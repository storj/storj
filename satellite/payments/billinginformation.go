// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

// TaxCountry is a country that Stripe supports for tax reporting.
type TaxCountry struct {
	Name string `json:"name"`
	Code string `json:"code"`
}

// BillingAddress contains a user's custom billing address.
type BillingAddress struct {
	Name       string     `json:"name"`
	Line1      string     `json:"line1"`
	Line2      string     `json:"line2"`
	City       string     `json:"city"`
	PostalCode string     `json:"postalCode"`
	State      string     `json:"state"`
	Country    TaxCountry `json:"country"`
}

// BillingInformation contains a user's billing information.
type BillingInformation struct {
	Address *BillingAddress `json:"address"`
}
