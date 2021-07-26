// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/zeebo/errs"
)

const recaptchaAPIURL = "https://www.google.com/recaptcha/api/siteverify"

// RecaptchaHandler is responsible for contacting the reCAPTCHA API
// and returning whether the user response characterized by the given
// response token and IP is valid.
type RecaptchaHandler interface {
	Verify(ctx context.Context, responseToken string, userIP string) (bool, error)
}

// googleRecaptcha is a reCAPTCHA handler that contacts Google's reCAPTCHA API.
type googleRecaptcha struct {
	SecretKey string
}

// NewDefaultRecaptcha returns a reCAPTCHA handler that contacts Google's reCAPTCHA API.
func NewDefaultRecaptcha(secretKey string) RecaptchaHandler {
	return googleRecaptcha{SecretKey: secretKey}
}

// Verify contacts the reCAPTCHA API and returns whether the given response token is valid.
// The documentation can be found here: https://developers.google.com/recaptcha/docs/verify
func (r googleRecaptcha) Verify(ctx context.Context, responseToken string, userIP string) (valid bool, err error) {
	if responseToken == "" {
		return false, errs.New("the response token is empty")
	}
	if userIP == "" {
		return false, errs.New("the user's IP address is empty")
	}

	reqBody := url.Values{
		"secret":   {r.SecretKey},
		"response": {responseToken},
		"remoteip": {userIP},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, recaptchaAPIURL, strings.NewReader(reqBody))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return false, errors.New(resp.Status)
	}

	var data struct {
		Success bool `json:"success"`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return false, err
	}

	return data.Success, nil
}
