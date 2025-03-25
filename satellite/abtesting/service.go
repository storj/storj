// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package abtesting

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/console"
)

// Error - console ab testing error type.
var Error = errs.Class("consoleapi ab testing error")

// Config contains configurations for the AB testing service.
type Config struct {
	Enabled        bool   `help:"whether or not AB testing is enabled" default:"false"`
	APIKey         string `help:"the Flagship API key"`
	EnvId          string `help:"the Flagship environment ID"`
	FlagshipURL    string `help:"the Flagship API URL" default:"https://decision.flagship.io/v2"`
	HitTrackingURL string `help:"the Flagship environment ID" default:"https://ariane.abtasty.com"`
}

// ABService is an interface for AB test methods.
type ABService interface {
	// GetABValues gets AB test values for a specific user. It returns a default value on error.
	GetABValues(ctx context.Context, user console.User) (values map[string]interface{}, err error)
	// SendHit sends an "action" event to flagship.
	SendHit(ctx context.Context, user console.User, action string)
}

// Service is a service that exposes all ab testing functionality.
type Service struct {
	log    *zap.Logger
	Config Config
}

// NewService is a constructor for AB service.
func NewService(log *zap.Logger, config Config) *Service {
	return &Service{
		log:    log,
		Config: config,
	}
}

// GetABValues gets AB test values for a specific user.
func (s *Service) GetABValues(ctx context.Context, user console.User) (values map[string]interface{}, err error) {
	reqBody, err := json.Marshal(map[string]interface{}{
		"visitor_id": user.ID,
	})
	if err != nil {
		err = Error.Wrap(err)
		s.log.Error("failed to encode request body", zap.Error(err))
		return nil, err
	}

	// We're getting all AB test campaigns for a user.
	// see: https://docs.developers.flagship.io/docs/decision-api#campaigns.
	path := fmt.Sprintf("%s/%s/campaigns", s.Config.FlagshipURL, s.Config.EnvId)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(reqBody))
	if err != nil {
		err = Error.Wrap(err)
		s.log.Error("flagship: failed to communicate with API", zap.Error(err))
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.Config.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		err = Error.Wrap(err)
		s.log.Error("flagship: failed to communicate with API", zap.Error(err))
		return nil, err
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			s.log.Error("failed to close response body", zap.Error(Error.Wrap(err)))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		s.log.Error("flagship: hit request response is not OK", zap.String("Status", resp.Status))
		return nil, err
	}

	var campaigns struct {
		Campaigns []struct {
			Variation struct {
				Modifications struct {
					Value map[string]interface{} `json:"value"`
				} `json:"modifications"`
			} `json:"variation"`
		} `json:"campaigns"`
	}
	err = json.NewDecoder(resp.Body).Decode(&campaigns)
	if err != nil {
		s.log.Error("flagship: failed to decode response; returning default", zap.Error(Error.Wrap(err)))
		return nil, err
	}

	values = make(map[string]interface{})
	for _, c := range campaigns.Campaigns {
		for k, val := range c.Variation.Modifications.Value {
			values[k] = val
		}
	}

	return values, nil
}

// SendHit sends an "action" event to flagship.
func (s *Service) SendHit(ctx context.Context, user console.User, action string) {
	// https://docs.developers.flagship.io/docs/universal-collect-documentation
	reqBody, err := json.Marshal(map[string]interface{}{
		"cid": s.Config.EnvId,
		"vid": user.ID,
		"ea":  action,
		"ec":  "Action Tracking",
		"ds":  "APP",
		"t":   "EVENT",
	})
	if err != nil {
		s.log.Error("failed to encode hit json request", zap.Error(Error.Wrap(err)))
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.Config.HitTrackingURL, bytes.NewReader(reqBody))
	if err != nil {
		s.log.Error("flagship: failed to send hit", zap.Error(Error.Wrap(err)))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.log.Error("flagship: failed to send hit", zap.Error(Error.Wrap(err)))
		return
	}
	defer func() {
		err = resp.Body.Close()
		if err != nil {
			s.log.Error("failed to close response body", zap.Error(Error.Wrap(err)))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		s.log.Error("flagship: hit request response is not OK", zap.String("Status", resp.Status))
		return
	}
}
