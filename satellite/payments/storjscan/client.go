// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package storjscan

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/currency"
	"storj.io/storj/private/blockchain"
)

var (
	// ClientErr is general purpose storjscan client error class.
	ClientErr = errs.Class("storjscan client")
	// ClientErrUnauthorized is unauthorized err storjscan client error class.
	ClientErrUnauthorized = errs.Class("storjscan client unauthorized")
)

// Header holds ethereum blockchain block header data.
type Header struct {
	ChainID   int64
	Hash      blockchain.Hash
	Number    int64
	Timestamp time.Time
}

// Payment holds storjscan payment data.
type Payment struct {
	ChainID     int64
	From        blockchain.Address
	To          blockchain.Address
	TokenValue  currency.Amount
	USDValue    currency.Amount
	BlockHash   blockchain.Hash
	BlockNumber int64
	Transaction blockchain.Hash
	LogIndex    int
	Timestamp   time.Time
}

// LatestPayments contains latest payments and latest chain block header.
type LatestPayments struct {
	LatestBlocks []Header
	Payments     []Payment
}

// Client is storjscan HTTP API client.
type Client struct {
	endpoint   string
	identifier string
	secret     string
	http       http.Client
}

// NewClient creates new storjscan API client.
func NewClient(endpoint, identifier, secret string) *Client {
	return &Client{
		endpoint:   endpoint,
		identifier: identifier,
		secret:     secret,
		http:       http.Client{},
	}
}

// AllPayments retrieves all payments after specified block for wallets associated with particular API key.
func (client *Client) AllPayments(ctx context.Context, from map[int64]int64) (payments LatestPayments, err error) {
	defer mon.Task()(&ctx)(&err)

	p := client.endpoint + "/api/v0/tokens/payments"

	payments, err = client.getPayments(ctx, p, from)

	return
}

// Payments retrieves payments after specified block for given address associated with particular API key.
func (client *Client) Payments(ctx context.Context, from map[int64]int64, address string) (payments LatestPayments, err error) {
	defer mon.Task()(&ctx)(&err)

	p := client.endpoint + "/api/v0/tokens/payments/" + address

	payments, err = client.getPayments(ctx, p, from)

	return
}

func (client *Client) getPayments(ctx context.Context, path string, from map[int64]int64) (_ LatestPayments, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return LatestPayments{}, ClientErr.Wrap(err)
	}

	req.SetBasicAuth(client.identifier, client.secret)

	query := req.URL.Query()
	for k, v := range from {
		query.Set(strconv.FormatInt(k, 10), strconv.FormatInt(v, 10))
	}
	req.URL.RawQuery = query.Encode()

	resp, err := client.http.Do(req)
	if err != nil {
		return LatestPayments{}, ClientErr.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, ClientErr.Wrap(resp.Body.Close()))
	}()

	if resp.StatusCode != http.StatusOK {
		var data struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return LatestPayments{}, ClientErr.Wrap(err)
		}

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return LatestPayments{}, ClientErrUnauthorized.New("%s", data.Error)
		default:
			return LatestPayments{}, ClientErr.New("%s", data.Error)
		}
	}

	var payments LatestPayments
	if err := json.NewDecoder(resp.Body).Decode(&payments); err != nil {
		return LatestPayments{}, ClientErr.Wrap(err)
	}

	return payments, nil
}

// ClaimNewEthAddress claims a new ethereum wallet address for the given user.
func (client *Client) ClaimNewEthAddress(ctx context.Context) (_ blockchain.Address, err error) {
	defer mon.Task()(&ctx)(&err)

	p := client.endpoint + "/api/v0/wallets/claim"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p, nil)
	if err != nil {
		return blockchain.Address{}, ClientErr.Wrap(err)
	}

	req.SetBasicAuth(client.identifier, client.secret)

	resp, err := client.http.Do(req)
	if err != nil {
		return blockchain.Address{}, ClientErr.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, ClientErr.Wrap(resp.Body.Close()))
	}()

	if resp.StatusCode != http.StatusOK {
		var data struct {
			Error string `json:"error"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return blockchain.Address{}, ClientErr.Wrap(err)
		}

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return blockchain.Address{}, ClientErrUnauthorized.New("%s", data.Error)
		default:
			return blockchain.Address{}, ClientErr.New("%s", data.Error)
		}
	}

	var address blockchain.Address

	if err = json.NewDecoder(resp.Body).Decode(&address); err != nil {
		return blockchain.Address{}, ClientErr.Wrap(err)
	}

	return address, nil
}
