// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package payments

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

// TaxID contains a user's tax information.
type TaxID struct {
	ID    string `json:"id"`
	Tax   Tax    `json:"tax"`
	Value string `json:"value"`
}

// BillingInformation contains a user's billing information.
type BillingInformation struct {
	Address          *BillingAddress `json:"address"`
	TaxIDs           []TaxID         `json:"taxIDs"`
	InvoiceReference string          `json:"invoiceReference"`
}
