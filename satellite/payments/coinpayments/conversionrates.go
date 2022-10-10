// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"storj.io/common/currency"
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
	RateBTC    decimal.Decimal
	TXFee      decimal.Decimal
	Status     ExchangeStatus
	LastUpdate time.Time
}

// UnmarshalJSON converts JSON string to currency rate info.
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

	rateBTC, err := decimal.NewFromString(rateRaw.RateBTC)
	if err != nil {
		return err
	}
	txFee, err := decimal.NewFromString(rateRaw.TXFee)
	if err != nil {
		return err
	}

	lastUpdate, err := strconv.ParseInt(rateRaw.LastUpdate, 10, 64)
	if err != nil {
		return err
	}

	*rateInfo = CurrencyRateInfo{
		IsFiat:     rateRaw.IsFiat > 0,
		RateBTC:    rateBTC,
		TXFee:      txFee,
		Status:     ExchangeStatus(rateRaw.Status),
		LastUpdate: time.Unix(lastUpdate, 0),
	}

	return nil
}

// CurrencyRateInfos maps currency to currency rate info.
type CurrencyRateInfos map[CurrencySymbol]CurrencyRateInfo

// ForCurrency allows lookup into a CurrencyRateInfos map by currency
// object, instead of by its coinpayments.net-specific symbol.
func (infos CurrencyRateInfos) ForCurrency(currency *currency.Currency) (info CurrencyRateInfo, ok bool) {
	coinpaymentsSymbol, ok := currencySymbols[currency]
	if !ok {
		return info, false
	}
	info, ok = infos[coinpaymentsSymbol]
	return info, ok
}

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
