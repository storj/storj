// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package coinpayments

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/zeebo/errs"
)

// Error is error class API errors.
var Error = errs.Class("coinpayments client error")

// Credentials contains public and private API keys for client.
type Credentials struct {
	PublicKey  string
	PrivateKey string
}

// Client handles base API processing.
type Client struct {
	creds Credentials
	http  http.Client
}

// NewClient creates new instance of client with provided credentials.
func NewClient(creds Credentials) *Client {
	client := &Client{
		creds: creds,
		http: http.Client{
			Timeout: 0,
		},
	}
	return client
}

// Transactions returns transactions API.
func (c *Client) Transactions() Transactions {
	return Transactions{client: c}
}

// do handles base API request routines.
func (c *Client) do(ctx context.Context, cmd string, values url.Values) (_ json.RawMessage, err error) {
	values.Set("version", "1")
	values.Set("format", "json")
	values.Set("key", c.creds.PublicKey)
	values.Set("cmd", cmd)

	encoded := values.Encode()

	buff := bytes.NewBufferString(encoded)

	req, err := http.NewRequest(http.MethodPost, "https://www.coinpayments.net/api.php", buff)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HMAC", c.hmac([]byte(encoded)))

	resp, err := c.http.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, errs.New("internal server error")
	}

	var data struct {
		Error  string          `json:"error"`
		Result json.RawMessage `json:"result"`
	}

	if err = json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if data.Error != "ok" {
		return nil, errs.New(data.Error)
	}

	return data.Result, nil
}

// hmac returns string representation of HMAC signature
// signed with clients private key.
func (c *Client) hmac(payload []byte) string {
	mac := hmac.New(sha512.New, []byte(c.creds.PrivateKey))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}
