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
const hcaptchaAPIURL = "https://hcaptcha.com/siteverify"

// CaptchaHandler is responsible for contacting a captcha API
// and returning whether the user response characterized by the given
// response token and IP is valid.
type CaptchaHandler interface {
	Verify(ctx context.Context, responseToken string, userIP string) (bool, *float64, error)
}

// CaptchaType is a type of captcha.
type CaptchaType int

const (
	// Recaptcha is the type for reCAPTCHA.
	Recaptcha CaptchaType = iota
	// Hcaptcha is the type for hCaptcha.
	Hcaptcha
)

// captchaHandler is a captcha handler that contacts a reCAPTCHA or hCaptcha API.
type captchaHandler struct {
	SecretKey string
	Endpoint  string
}

// NewDefaultCaptcha returns a captcha handler that contacts a reCAPTCHA or hCaptcha API.
func NewDefaultCaptcha(kind CaptchaType, secretKey string) CaptchaHandler {
	handler := captchaHandler{SecretKey: secretKey}
	switch kind {
	case Recaptcha:
		handler.Endpoint = recaptchaAPIURL
	case Hcaptcha:
		handler.Endpoint = hcaptchaAPIURL
	}
	return handler
}

// Verify contacts the captcha API and returns whether the given response token is valid.
// The documentation can be found here for recaptcha: https://developers.google.com/recaptcha/docs/verify
// And here for hcaptcha: https://docs.hcaptcha.com/
func (r captchaHandler) Verify(ctx context.Context, responseToken string, userIP string) (valid bool, score *float64, err error) {
	if responseToken == "" {
		return false, nil, errs.New("the response token is empty")
	}
	if userIP == "" {
		return false, nil, errs.New("the user's IP address is empty")
	}

	reqBody := url.Values{
		"secret":   {r.SecretKey},
		"response": {responseToken},
		"remoteip": {userIP},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.Endpoint, strings.NewReader(reqBody))
	if err != nil {
		return false, nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil, err
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return false, nil, errors.New(resp.Status)
	}

	var data struct {
		Success bool    `json:"success"`
		Score   float64 `json:"score"`
	}
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return false, nil, err
	}

	return data.Success, &data.Score, nil
}
