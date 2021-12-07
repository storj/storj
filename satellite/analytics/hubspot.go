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

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

var mon = monkit.Package()

// HubSpotConfig is a configuration struct for Concurrent Sending of Events.
type HubSpotConfig struct {
	APIKey          string `help:"hubspot api key" default:""`
	ChannelSize     int    `help:"the number of events that can be in the queue before dropping" default:"10000"`
	ConcurrentSends int    `help:"creates a new limiter with limit set to" default:"25"`
}

// Event is a configuration struct for sending API request to HubSpot.
type Event struct {
	Data     map[string]interface{}
	Endpoint string
}

// HubspotEvents is a configuration struct for sending Events data to HubSpot.
type HubspotEvents struct {
	log           *zap.Logger
	config        HubSpotConfig
	events        chan []Event
	apiKey        string
	satelliteName string
	worker        *sync2.Limiter
}

// NewHubSpotEvents for sending user events to HubSpot.
func NewHubSpotEvents(config HubSpotConfig, satelliteName string) *HubspotEvents {
	return &HubspotEvents{
		config:        config,
		events:        make(chan []Event, config.ChannelSize),
		apiKey:        url.QueryEscape(config.APIKey),
		satelliteName: satelliteName,
		worker:        sync2.NewLimiter(config.ConcurrentSends),
	}
}

// Run for concurrent API requests.
func (q *HubspotEvents) Run(ctx context.Context) error {
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
				q.Handle(ctx, ev)
			})
		}
	}
}

// EnqueueCreateUser for creating user in HubSpot.
func (q *HubspotEvents) EnqueueCreateUser(fields TrackCreateUserFields) {

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

	createUser := Event{
		Endpoint: "https://api.hubapi.com/crm/v3/objects/contacts?hapikey=" + q.apiKey,
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

	sendUserEvent := Event{
		Endpoint: "https://api.hubapi.com/events/v3/send?hapikey=" + q.apiKey,
		Data: map[string]interface{}{
			"email":     fields.Email,
			"eventName": "pe20293085_account_created_new",
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
	case q.events <- []Event{createUser, sendUserEvent}:
	default:
	}
}

// EnqueueEvent for sending user behavioral event to HubSpot.
func (q *HubspotEvents) EnqueueEvent(email, eventName string, properties map[string]interface{}) {

	newEvent := Event{
		Endpoint: "https://api.hubapi.com/events/v3/send?hapikey=" + q.apiKey,
		Data: map[string]interface{}{
			"email":      email,
			"eventName":  eventName,
			"properties": properties,
		},
	}
	select {
	case q.events <- []Event{newEvent}:
	default:
	}
}

// Handle for handle the HubSpot API requests.
func (q *HubspotEvents) Handle(ctx context.Context, events []Event) {
	var err error
	defer mon.Task()(&ctx)(&err)
	for _, ev := range events {
		payloadBytes, err := json.Marshal(ev.Data)
		if err != nil {
			q.log.Error("Error in converting into bytes", zap.Error(err))
		}

		req, err := http.NewRequestWithContext(ctx, "POST", ev.Endpoint, bytes.NewReader(payloadBytes))
		if err != nil {
			q.log.Error("Error in returning a new request", zap.Error(err))
		}

		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			q.log.Error("Error making a request with specified context", zap.Error(err))
		}

		defer func() {
			err = errs.Combine(err, resp.Body.Close())
		}()
	}
}
