// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
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
	ConcurrentSends int           `help:"the number of concurrent api requests that can be made" default:"4"`
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
func NewHubSpotEvents(log *zap.Logger, config HubSpotConfig, satelliteName string) *HubSpotEvents {
	return &HubSpotEvents{
		log:           log,
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
					q.log.Error("Sending hubspot event", zap.Error(err))
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

	newField := func(name, value string) map[string]interface{} {
		return map[string]interface{}{
			"name":  name,
			"value": value,
		}
	}

	createUser := HubSpotEvent{
		Endpoint: "https://api.hsforms.com/submissions/v3/integration/submit/20293085/77cfa709-f533-44b8-bf3a-ed1278ca3202?hapikey=" + q.escapedAPIKey,
		Data: map[string]interface{}{
			"context": map[string]interface{}{
				"hutk": fields.HubspotUTK,
			},
			"fields": []map[string]interface{}{
				newField("email", fields.Email),
				newField("firstname", firstName),
				newField("lastname", lastName),
				newField("lifecyclestage", "other"),
				newField("origin_header", fields.OriginHeader),
				newField("signup_referrer", fields.Referrer),
				newField("account_created", "true"),
				newField("have_sales_contact", strconv.FormatBool(fields.HaveSalesContact)),
				newField("signup_partner", fields.UserAgent),
			},
		},
	}

	sendUserEvent := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/events/v3/send?hapikey=" + q.escapedAPIKey,
		Data: map[string]interface{}{
			"email":     fields.Email,
			"eventName": eventPrefix + "_" + strings.ToLower(q.satelliteName) + "_" + "account_created",
			"properties": map[string]interface{}{
				"userid":             fields.ID.String(),
				"email":              fields.Email,
				"name":               fields.FullName,
				"satellite_selected": q.satelliteName,
				"account_type":       string(fields.Type),
				"company_size":       fields.EmployeeCount,
				"company_name":       fields.CompanyName,
				"job_title":          fields.JobTitle,
			},
		},
	}
	select {
	case q.events <- []HubSpotEvent{createUser, sendUserEvent}:
	default:
		q.log.Error("create user hubspot event failed, event channel is full")
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
		q.log.Error("sending hubspot event failed, event channel is full")
	}
}

// handleSingleEvent for handle the single HubSpot API request.
func (q *HubSpotEvents) handleSingleEvent(ctx context.Context, ev HubSpotEvent) (err error) {
	payloadBytes, err := json.Marshal(ev.Data)
	if err != nil {
		return Error.New("json marshal failed: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ev.Endpoint, bytes.NewReader(payloadBytes))
	if err != nil {
		return Error.New("new request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return Error.New("send request failed: %w", err)
	}

	err = resp.Body.Close()
	if err != nil {
		err = Error.New("closing resp body failed: %w", err)
	}
	return err
}

// Handle for handle the HubSpot API requests.
func (q *HubSpotEvents) Handle(ctx context.Context, events []HubSpotEvent) (err error) {
	defer mon.Task()(&ctx)(&err)
	for _, ev := range events {
		err := q.handleSingleEvent(ctx, ev)
		if err != nil {
			return Error.New("handle event: %w", err)
		}
	}
	return nil
}
