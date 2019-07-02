// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gorilla/schema"

	"storj.io/storj/internal/currency"
	"storj.io/storj/satellite/rewards"
)

// parseOfferForm decodes POST form data into a new offer.
func parseOfferForm(w http.ResponseWriter, req *http.Request) (rewards.NewOffer, error) {
	err := req.ParseForm()
	if err != nil {
		return rewards.NewOffer{}, err
	}

	var offer rewards.NewOffer
	err = decoder.Decode(&offer, req.PostForm)
	return offer, err
}

var (
	decoder = schema.NewDecoder()
)

// init safely registers convertStringToTime for the decoder.
func init() {
	decoder.RegisterConverter(time.Time{}, convertStringToTime)
	decoder.RegisterConverter(currency.USD{}, convertStringToUSD)
}

// convertStringToUSD formats dollars strings as USD amount.
func convertStringToUSD(s string) reflect.Value {
	value, err := strconv.Atoi(s)
	if err != nil {
		// invalid decoder value
		return reflect.Value{}
	}
	return reflect.ValueOf(currency.Dollars(value))
}

// convertStringToTime formats form time input as time.Time.
func convertStringToTime(value string) reflect.Value {
	v, err := time.Parse("2006-01-02", value)
	if err != nil {
		// invalid decoder value
		return reflect.Value{}
	}
	return reflect.ValueOf(v)
}
