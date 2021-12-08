// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

var mon = monkit.Package()

const (
	eventPrefix = "pe20293085"
)

// HubSpotConfig is a configuration struct for Concurrent Sending of Events.
type HubSpotConfig struct {
	APIKey          string        `help:"hubspot api key" default:""`
	ChannelSize     int           `help:"the number of events that can be in the queue before dropping" default:"1000"`
	ConcurrentSends int           `help:"creates a new limiter with limit set to" default:"4"`
	DefaultTimeout  time.Duration `help:"the default timeout for the hubspot http client" default:"10s"`
}

// HubSpotEvent is a configuration struct for sending API request to HubSpot.
type HubSpotEvent struct {
	Data     map[string]interface{}
	Endpoint string
}

// HubSpotEvents is a configuration struct for sending Events data to HubSpot.
type HubSpotEvents struct {
	log           *zap.Logger
	config        HubSpotConfig
	events        chan []HubSpotEvent
	escapedAPIKey string
	satelliteName string
	worker        sync2.Limiter
	httpClient    *http.Client
}

// NewHubSpotEvents for sending user events to HubSpot.
func NewHubSpotEvents(config HubSpotConfig, satelliteName string) *HubSpotEvents {
	return &HubSpotEvents{
		config:        config,
		events:        make(chan []HubSpotEvent, config.ChannelSize),
		escapedAPIKey: url.QueryEscape(config.APIKey),
		satelliteName: satelliteName,
		worker:        *sync2.NewLimiter(config.ConcurrentSends),
		httpClient: &http.Client{
			Timeout: config.DefaultTimeout,
		},
	}
}

// Run for concurrent API requests.
func (q *HubSpotEvents) Run(ctx context.Context) error {
	defer q.worker.Wait()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-q.events:
			q.worker.Go(ctx, func() {
				err := q.Handle(ctx, ev)
				if err != nil {
					q.log.Error("Sending hubspot event API request failed")
				}
			})
		}
	}
}

// EnqueueCreateUser for creating user in HubSpot.
func (q *HubSpotEvents) EnqueueCreateUser(fields TrackCreateUserFields) {
	fullName := fields.FullName
	names := strings.SplitN(fullName, " ", 2)

	var firstName string
	var lastName string

	if len(names) > 1 {
		firstName = names[0]
		lastName = names[1]
	} else {
		firstName = fullName
	}

	createUser := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/crm/v3/objects/contacts?hapikey=" + q.escapedAPIKey,
		Data: map[string]interface{}{
			"email": fields.Email,
			"properties": map[string]interface{}{
				"email":          fields.Email,
				"firstname":      firstName,
				"lastname":       lastName,
				"lifecyclestage": "customer",
			},
		},
	}

	sendUserEvent := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/events/v3/send?hapikey=" + q.escapedAPIKey,
		Data: map[string]interface{}{
			"email":     fields.Email,
			"eventName": eventPrefix + "_" + "account_created_new",
			"properties": map[string]interface{}{
				"userid":             fields.ID.String(),
				"email":              fields.Email,
				"name":               fields.FullName,
				"satellite_selected": q.satelliteName,
				"account_type":       string(fields.Type),
				"company_size":       fields.EmployeeCount,
				"company_name":       fields.CompanyName,
				"job_title":          fields.JobTitle,
				"have_sales_contact": fields.HaveSalesContact,
			},
		},
	}
	select {
	case q.events <- []HubSpotEvent{createUser, sendUserEvent}:
	default:
		q.log.Error("Create user in HubSpot failed")
	}
}

// EnqueueEvent for sending user behavioral event to HubSpot.
func (q *HubSpotEvents) EnqueueEvent(email, eventName string, properties map[string]interface{}) {
	eventName = strings.ReplaceAll(eventName, " ", "_")
	eventName = strings.ToLower(eventName)
	eventName = eventPrefix + "_" + eventName

	newEvent := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/events/v3/send?hapikey=" + q.escapedAPIKey,
		Data: map[string]interface{}{
			"email":      email,
			"eventName":  eventName,
			"properties": properties,
		},
	}
	select {
	case q.events <- []HubSpotEvent{newEvent}:
	default:
		q.log.Error("Sending user event in HubSpot failed")
	}
}

// handleSingleEvent for handle the single HubSpot API request.
func (q *HubSpotEvents) handleSingleEvent(ctx context.Context, ev HubSpotEvent) (err error) {
	payloadBytes, err := json.Marshal(ev.Data)
	if err != nil {
		return errs.Wrap(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ev.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return errs.Wrap(err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return errs.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	return nil
}

// Handle for handle the HubSpot API requests.
func (q *HubSpotEvents) Handle(ctx context.Context, events []HubSpotEvent) (err error) {
	defer mon.Task()(&ctx)(&err)
	for _, ev := range events {
		err := q.handleSingleEvent(ctx, ev)
		if err != nil {
			return errs.Wrap(err)
		}
	}
	return nil
}
