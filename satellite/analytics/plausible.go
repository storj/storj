// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
)

// plausibleConfig is a configuration struct for plausible analytics.
type plausibleConfig struct {
	Domain         string        `help:"the domain set up on plausible for the satellite" default:""`
	ApiUrl         string        `help:"the url of the plausible API" releaseDefault:"https://plausible.io/api/event" devDefault:""`
	DefaultTimeout time.Duration `help:"the default timeout for the plausible http client" default:"10s"`
}

// PageViewBody is a struct for page view event.
type PageViewBody struct {
	Name      string            `json:"name"`
	Url       string            `json:"url"`
	Props     map[string]string `json:"props"`
	Domain    string            `json:"domain"`
	Referrer  string            `json:"referrer"`
	IP        string            `json:"-"`
	UserAgent string            `json:"-"`
}

// PlausibleService is a configuration struct for sending data to Plausible.
type plausibleService struct {
	log        *zap.Logger
	config     plausibleConfig
	httpClient *http.Client
}

// newPlausibleService for sending events to Plausible.
func newPlausibleService(log *zap.Logger, config plausibleConfig) *plausibleService {
	return &plausibleService{
		log:    log,
		config: config,
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
	}
}

// pageViewEvent sends a page view event to plausible.
func (pS *plausibleService) pageViewEvent(ctx context.Context, pv PageViewBody) error {
	if pS.config.Domain == "" || pS.config.ApiUrl == "" {
		return nil
	}

	pv.Name = "pageview"
	pv.Domain = pS.config.Domain
	payloadBytes, err := json.Marshal(pv)
	if err != nil {
		return Error.New("json marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, pS.config.ApiUrl, bytes.NewReader(payloadBytes))
	if err != nil {
		return Error.New("new request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", pv.UserAgent)
	req.Header.Set("X-Forwarded-For", pv.IP)
	resp, err := pS.httpClient.Do(req)
	if err != nil {
		pS.log.Error("send request failed", zap.Error(err))
		return Error.New("send request failed: %w", err)
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusAccepted {
		pS.log.Error("send request failed", zap.Error(err))
		return Error.New("failed to send plausible event")
	}

	return err
}
