// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import "net/url"

// GetCheckoutURL constructs checkout url from auth key and transaction id.
func GetCheckoutURL(key string, id TransactionID) string {
	u, _ := url.Parse("https://coinpayments.net/index.php")

	query := u.Query()
	query.Add("cmd", "checkout")
	query.Add("id", id.String())
	query.Add("key", key)

	u.RawQuery = query.Encode()
	return u.String()
}
