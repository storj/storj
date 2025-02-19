// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package valdiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"github.com/zeebo/errs"
)

// UserPath is the path for storj users.
const UserPath = "storj/account"

// UserCreationData contains necessary data to create a storj user in Valdi.
type UserCreationData struct {
	Email    string `json:"email"`
	Username string `json:"username"`
	Country  string `json:"country"`
}

// CreateUser creates a user in Valdi.
func (c *Client) CreateUser(ctx context.Context, createUserData UserCreationData) (statusCode int, err error) {
	defer mon.Task()(&ctx)(&err)

	url, err := url.JoinPath(c.baseURL, UserPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	jsonData, err := json.Marshal(createUserData)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return http.StatusInternalServerError, err
	}

	token := c.createJWT(createUserData.Email)

	if c.signRequests {
		if c.privateKey == nil {
			return http.StatusInternalServerError, errs.New("private key is nil")
		}
		tokenString, err := token.SignedString(c.privateKey)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		req.Header.Set("Authorization", "Bearer "+tokenString)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode == http.StatusCreated {
		return resp.StatusCode, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	var errorResp ErrorMessage
	if err := json.Unmarshal(body, &errorResp); err != nil {
		return http.StatusInternalServerError, err
	}
	err = errors.New(errorResp.Detail)
	return resp.StatusCode, err
}
