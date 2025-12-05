// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package hubspotmails

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/context2"
	"storj.io/storj/satellite/analytics"
)

var (
	// Error is the base error for the hubspot mail service.
	Error = errs.Class("hubspot mail service")

	mon = monkit.Package()
)

// MailKind represents the type of the email.
type MailKind string

var (
	// GhostSessionWarning is sent when a ghost session is detected.
	GhostSessionWarning MailKind = "ghostSessionWarning"
)

// SendEmailRequest is a request to send an email via HubSpot.
type SendEmailRequest struct {
	Kind    MailKind
	To      string
	From    string   // optional; if empty, HS template's From is used.
	SendID  string   // optional idempotency key (HubSpot de-dupes).
	ReplyTo []string // optional.
	CC      []string // optional.
	BCC     []string // optional.

	Contact map[string]any // persisted on contact (optional).
	Custom  map[string]any // template-only vars (optional).
}

// Service sends emails using the HubSpot API.
//
// architecture: Service
type Service struct {
	log        *zap.Logger
	httpClient *http.Client
	config     Config

	analytics *analytics.Service

	sending sync.WaitGroup
}

// NewService creates a new Service instance.
func NewService(log *zap.Logger, analytics *analytics.Service, config Config) *Service {
	return &Service{
		log:    log,
		config: config,
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
		analytics: analytics,
	}
}

// Close closes and waits for any pending actions.
func (ms *Service) Close() error {
	ms.sending.Wait()
	return nil
}

// SendAsync sends an email asynchronously using the HubSpot API.
func (ms *Service) SendAsync(ctx context.Context, req *SendEmailRequest) {
	var err error
	defer mon.Task()(&ctx)(&err)

	if !ms.config.Enabled {
		ms.log.Warn("hubspot email service is disabled, skipping email send",
			zap.String("kind", string(req.Kind)),
			zap.String("to", req.To))
		return
	}

	token, err := ms.analytics.GetAccessToken(ctx)
	if err != nil {
		ms.log.Error("failed to get access token for HubSpot email service",
			zap.Error(err),
			zap.String("kind", string(req.Kind)),
			zap.String("to", req.To))
		return
	}

	ms.sending.Add(1)
	go func() {
		defer ms.sending.Done()

		ctx, cancel := context.WithTimeout(context2.WithoutCancellation(ctx), 5*time.Second)
		defer cancel()

		if err := ms.send(ctx, req, token); err != nil {
			ms.log.Error("email send failed",
				zap.Error(err),
				zap.String("kind", string(req.Kind)),
				zap.String("to", req.To))
			return
		}
		ms.log.Info("email sent",
			zap.String("kind", string(req.Kind)),
			zap.String("to", req.To))
	}()
}

type hsRequest struct {
	EmailID      int64          `json:"emailId"`
	Message      map[string]any `json:"message"`
	ContactProps map[string]any `json:"contactProperties,omitempty"`
	CustomProps  map[string]any `json:"customProperties,omitempty"`
}
type hsError struct {
	Message string `json:"message"`
	In      string `json:"in"`
}
type hsResponseData struct {
	Message string    `json:"message"`
	Errors  []hsError `json:"errors"`
}

func (ms *Service) send(ctx context.Context, r *SendEmailRequest, token string) error {
	emailID, ok := ms.config.EmailKindIDMap.kindIDMap[r.Kind]
	if !ok || emailID == 0 {
		return Error.New("hubspot: missing emailId for kind=%s", r.Kind)
	}
	if r.To == "" {
		return Error.New("hubspot: message.to is required")
	}

	msg := map[string]any{"to": r.To}
	if r.From != "" {
		msg["from"] = r.From
	}
	if r.SendID != "" {
		msg["sendId"] = r.SendID
	}
	if len(r.ReplyTo) > 0 {
		msg["replyTo"] = r.ReplyTo
	}
	if len(r.CC) > 0 {
		msg["cc"] = r.CC
	}
	if len(r.BCC) > 0 {
		msg["bcc"] = r.BCC
	}

	payload, err := json.Marshal(hsRequest{
		EmailID:      emailID,
		Message:      msg,
		ContactProps: r.Contact,
		CustomProps:  r.Custom,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ms.config.SendEmailAPI, bytes.NewReader(payload))
	if err != nil {
		return Error.Wrap(err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := ms.httpClient.Do(req)
	if err != nil {
		return Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted {
		return nil
	}

	var data hsResponseData
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return Error.New("decoding response failed: %w", err)
	}

	return Error.New("sending email failed: %s - %v", data.Message, data.Errors)
}
