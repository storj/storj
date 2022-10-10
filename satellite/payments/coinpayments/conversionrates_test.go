// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"storj.io/common/currency"
	"storj.io/common/testcontext"
)

const (
	// reference https://www.coinpayments.net/apidoc-rates
	ratesJSON = `{
		"USD": {
			"is_fiat": 1,
			"rate_btc": "0.0001234215748657146721341",
			"last_update": "1633015701",
			"tx_fee": "0.00000000",
			"status": "online",
			"name": "United States Dollar",
			"confirms": "3",
			"capabilities": [
				"payments", "wallet", "transfers", "convert"
			]
		},
		"BTC": {
			"is_fiat": 0,
			"rate_btc": "1.000000000000000000000000",
			"last_update": "1632931502",
			"tx_fee": "0.00100000",
			"status": "online",
			"name": "Bitcoin",
			"confirms": "2",
			"capabilities": [
				"payments", "wallet", "transfers", "convert"
			]
		},
		"LTCT": {
			"is_fiat": 0,
			"rate_btc": "999999.999999999999999999",
			"last_update": "1628027418",
			"tx_fee": "0.00000000",
			"status": "online",
			"name": "LTCT test coins",
			"confirms": "2",
			"capabilities": []
		}
	}`

	resultJSON = `{"error": "ok", "result": ` + ratesJSON + `}`

	publicKey  = "hi i am a public key"
	privateKey = "hi i am a private key"
)

type dumbMockClient struct {
	response string
}

func (c *dumbMockClient) Do(req *http.Request) (*http.Response, error) {
	return &http.Response{
		Status:        "OK",
		StatusCode:    http.StatusOK,
		Body:          io.NopCloser(bytes.NewBuffer([]byte(c.response))),
		ContentLength: int64(len(c.response)),
	}, nil
}

func TestProcessingConversionRates(t *testing.T) {
	rateService := Client{
		creds: Credentials{PublicKey: publicKey, PrivateKey: privateKey},
		http:  &dumbMockClient{response: resultJSON},
	}

	rateInfos, err := rateService.ConversionRates().Get(testcontext.New(t))
	require.NoError(t, err)

	require.Truef(t, rateInfos["BTC"].RateBTC.Equal(decimal.NewFromFloat(1.0)),
		"expected 1.0, but got %v", rateInfos["BTC"].RateBTC.String())
	require.Truef(t, rateInfos["USD"].RateBTC.LessThan(decimal.NewFromInt(1)),
		"expected value less than 1, but got %v", rateInfos["USD"].RateBTC.String())

	rateInfo, ok := rateInfos.ForCurrency(currency.USDollars)
	require.True(t, ok)
	require.True(t, rateInfo.IsFiat)

	_, ok = rateInfos.ForCurrency(currency.LiveGoats)
	require.False(t, ok)

	rateInfo, ok = rateInfos.ForCurrency(CurrencyLTCT)
	require.True(t, ok)
	require.True(t, rateInfo.TXFee.Equal(decimal.NewFromInt(0)))
}
