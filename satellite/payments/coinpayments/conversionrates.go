// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"context"
	"encoding/json"
	"math/big"
	"net/url"
	"strconv"
	"time"
)

// cmdRates is API command for retrieving currency rate infos.
const cmdRates = "rates"

// ExchangeStatus defines if currency is exchangeable.
type ExchangeStatus string

const (
	// ExchangeStatusOnline defines exchangeable currency.
	ExchangeStatusOnline ExchangeStatus = "online"
	// ExchangeStatusOffline defines currency that can not be convertible at the moment.
	ExchangeStatusOffline ExchangeStatus = "offline"
)

// CurrencyRateInfo holds currency conversion info.
type CurrencyRateInfo struct {
	IsFiat     bool
	RateBTC    big.Float
	TXFee      big.Float
	Status     ExchangeStatus
	LastUpdate time.Time
}

// UnmarshalJSON converts JSON string to currency rate info,
func (rateInfo *CurrencyRateInfo) UnmarshalJSON(b []byte) error {
	var rateRaw struct {
		IsFiat     int    `json:"is_fiat"`
		RateBTC    string `json:"rate_btc"`
		TXFee      string `json:"tx_fee"`
		Status     string `json:"status"`
		LastUpdate string `json:"last_update"`
	}

	if err := json.Unmarshal(b, &rateRaw); err != nil {
		return err
	}

	parseBigFloat := func(s string) (*big.Float, error) {
		f, _, err := big.ParseFloat(s, 10, 256, big.ToNearestEven)
		return f, err
	}

	rateBTC, err := parseBigFloat(rateRaw.RateBTC)
	if err != nil {
		return err
	}
	txFee, err := parseBigFloat(rateRaw.TXFee)
	if err != nil {
		return err
	}

	lastUpdate, err := strconv.ParseInt(rateRaw.LastUpdate, 10, 64)
	if err != nil {
		return err
	}

	*rateInfo = CurrencyRateInfo{
		IsFiat:     rateRaw.IsFiat > 0,
		RateBTC:    *rateBTC,
		TXFee:      *txFee,
		Status:     ExchangeStatus(rateRaw.Status),
		LastUpdate: time.Unix(lastUpdate, 0),
	}

	return nil
}

// CurrencyRateInfos maps currency to currency rate info.
type CurrencyRateInfos map[Currency]CurrencyRateInfo

// ConversionRates collection of API methods for retrieving currency
// conversion rates.
type ConversionRates struct {
	client *Client
}

// Get returns USD rate for specified currency.
func (rates ConversionRates) Get(ctx context.Context) (CurrencyRateInfos, error) {
	values := make(url.Values)
	values.Set("short", "1")

	rateInfos := make(CurrencyRateInfos)

	res, err := rates.client.do(ctx, cmdRates, values)
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if err = json.Unmarshal(res, &rateInfos); err != nil {
		return nil, Error.Wrap(err)
	}

	return rateInfos, nil
}
