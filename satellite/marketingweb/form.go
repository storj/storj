// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gorilla/schema"

	"storj.io/storj/satellite/rewards"
)

// parseOfferForm decodes POST form data into a new offer.
func parseOfferForm(w http.ResponseWriter, req *http.Request) (o rewards.NewOffer, e error) {
	err := req.ParseForm()
	if err != nil {
		return o, err
	}

	if err := decoder.Decode(&o, req.PostForm); err != nil {
		return o, err
	}

	return o, nil
}

var (
	decoder = schema.NewDecoder()
)

// init safely registers convertStringToTime for the decoder.
func init() {
	decoder.RegisterConverter(time.Time{}, convertStringToTime)
	decoder.RegisterConverter(rewards.USD{}, convertStringToUSD)
}

// convertStringToUSD formats form time input as time.Time.
func convertStringToUSD(s string) reflect.Value {
	value, err := strconv.Atoi(s)
	if err != nil {
		// invalid decoder value
		return reflect.Value{}
	}
	return reflect.ValueOf(rewards.Dollars(int64(value)))
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
