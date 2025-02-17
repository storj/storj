// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package valdiclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/zeebo/errs"
)

// APIKeyPath is the path for storj api keys.
const APIKeyPath = "storj/apikey"

// CreateAPIKeyResponse is what Valdi returns when api key is created.
type CreateAPIKeyResponse struct {
	APIKey            string `json:"api_key"`
	SecretAccessToken string `json:"secret_access_token"`
}

// CreateAPIKey creates a valdi API key.
func (c *Client) CreateAPIKey(ctx context.Context, email string) (_ *CreateAPIKeyResponse, status int, err error) {
	defer mon.Task()(&ctx)(&err)

	url, err := url.JoinPath(c.baseURL, APIKeyPath)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	token := c.createJWT(email)

	if c.signRequests {
		if c.privateKey == nil {
			return nil, http.StatusInternalServerError, ErrPrivateKey.New("can't sign requests with nil key")
		}
		tokenString, err := token.SignedString(c.privateKey)
		if err != nil {
			return nil, http.StatusInternalServerError, ErrPrivateKey.New("error signing token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+tokenString)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	apiKey := &CreateAPIKeyResponse{}
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, apiKey); err != nil {
			return nil, http.StatusInternalServerError, err
		}
		return apiKey, resp.StatusCode, nil
	}

	var errorResp ErrorMessage
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return nil, http.StatusInternalServerError, err
	}
	err = errors.New(errorResp.Detail)
	return nil, resp.StatusCode, err
}
