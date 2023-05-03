// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package analytics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

var mon = monkit.Package()

const (
	eventPrefix      = "pe20293085"
	expiryBufferTime = 5 * time.Minute
	// string template for hubspot submission form. %s is a placeholder for the form(ID) being submitted.
	hubspotFormTemplate = "https://api.hsforms.com/submissions/v3/integration/submit/20293085/%s"
	// This form(ID) is the business account form.
	professionalFormID = "cc693502-9d55-4204-ae61-406a19148cfe"
	// This form(ID) is the personal account form.
	basicFormID = "77cfa709-f533-44b8-bf3a-ed1278ca3202"
	// The hubspot lifecycle stage of business accounts (Product Qualified Lead).
	businessLifecycleStage = "66198674"
	// The hubspot lifecycle stage of personal accounts.
	personalLifecycleStage = "other"
)

// HubSpotConfig is a configuration struct for Concurrent Sending of Events.
type HubSpotConfig struct {
	RefreshToken    string        `help:"hubspot refresh token" default:""`
	TokenAPI        string        `help:"hubspot token refresh API" default:"https://api.hubapi.com/oauth/v1/token"`
	ClientID        string        `help:"hubspot client ID" default:""`
	ClientSecret    string        `help:"hubspot client secret" default:""`
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
	log             *zap.Logger
	config          HubSpotConfig
	events          chan []HubSpotEvent
	refreshToken    string
	tokenAPI        string
	satelliteName   string
	worker          sync2.Limiter
	httpClient      *http.Client
	clientID        string
	clientSecret    string
	accessTokenData *TokenData
	mutex           sync.Mutex
}

// TokenData contains data related to the Hubspot access token.
type TokenData struct {
	AccessToken string
	ExpiresAt   time.Time
}

// NewHubSpotEvents for sending user events to HubSpot.
func NewHubSpotEvents(log *zap.Logger, config HubSpotConfig, satelliteName string) *HubSpotEvents {
	return &HubSpotEvents{
		log:           log,
		config:        config,
		events:        make(chan []HubSpotEvent, config.ChannelSize),
		refreshToken:  config.RefreshToken,
		tokenAPI:      config.TokenAPI,
		clientID:      config.ClientID,
		clientSecret:  config.ClientSecret,
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

	formFields := []map[string]interface{}{
		newField("email", fields.Email),
		newField("firstname", firstName),
		newField("lastname", lastName),
		newField("origin_header", fields.OriginHeader),
		newField("signup_referrer", fields.Referrer),
		newField("account_created", "true"),
		newField("have_sales_contact", strconv.FormatBool(fields.HaveSalesContact)),
		newField("signup_partner", fields.UserAgent),
	}

	properties := map[string]interface{}{
		"userid":             fields.ID.String(),
		"email":              fields.Email,
		"name":               fields.FullName,
		"satellite_selected": q.satelliteName,
		"account_type":       string(fields.Type),
		"company_size":       fields.EmployeeCount,
		"company_name":       fields.CompanyName,
		"job_title":          fields.JobTitle,
	}

	var formURL string

	if fields.Type == Professional {
		formFields = append(formFields, newField("lifecyclestage", businessLifecycleStage))
		formFields = append(formFields, newField("company", fields.CompanyName))
		formFields = append(formFields, newField("storage_needs", fields.StorageNeeds))

		properties["storage_needs"] = fields.StorageNeeds

		formURL = fmt.Sprintf(hubspotFormTemplate, professionalFormID)
	} else {
		formFields = append(formFields, newField("lifecyclestage", personalLifecycleStage))

		formURL = fmt.Sprintf(hubspotFormTemplate, basicFormID)
	}

	data := map[string]interface{}{
		"fields": formFields,
	}

	if fields.HubspotUTK != "" {
		data["context"] = map[string]interface{}{
			"hutk": fields.HubspotUTK,
		}
	}

	createUser := HubSpotEvent{
		Endpoint: formURL,
		Data:     data,
	}

	sendUserEvent := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/events/v3/send",
		Data: map[string]interface{}{
			"email":      fields.Email,
			"eventName":  eventPrefix + "_" + strings.ToLower(q.satelliteName) + "_" + "account_created",
			"properties": properties,
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
		Endpoint: "https://api.hubapi.com/events/v3/send",
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

	token, err := q.getAccessToken(ctx)
	if err != nil {
		return Error.New("token request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return Error.New("send request failed: %w", err)
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		var data struct {
			Message string `json:"message"`
		}
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return Error.New("decoding response failed: %w", err)
		}
		return Error.New("sending event failed: %s", data.Message)
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

// getAccessToken returns an access token for hubspot.
// It fetches a new token if there isn't one already or the old one is about to expire in expiryBufferTime.
// It locks q.mutex to ensure only one goroutine is able to request for a token.
func (q *HubSpotEvents) getAccessToken(ctx context.Context) (token string, err error) {
	defer mon.Task()(&ctx)(&err)

	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.accessTokenData == nil || q.accessTokenData.ExpiresAt.Add(-expiryBufferTime).Before(time.Now()) {
		q.accessTokenData, err = q.getAccessTokenFromHubspot(ctx)
		if err != nil {
			return "", err
		}
	}

	return q.accessTokenData.AccessToken, nil
}

// getAccessTokenFromHubspot gets a new access token from hubspot.
// Expects q.mutex to be locked.
func (q *HubSpotEvents) getAccessTokenFromHubspot(ctx context.Context) (_ *TokenData, err error) {
	defer mon.Task()(&ctx)(&err)

	values := make(url.Values)
	values.Set("grant_type", "refresh_token")
	values.Set("client_id", q.clientID)
	values.Set("client_secret", q.clientSecret)
	values.Set("refresh_token", q.refreshToken)

	encoded := values.Encode()

	buff := bytes.NewBufferString(encoded)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, q.tokenAPI, buff)
	if err != nil {
		return nil, Error.New("new request failed: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, Error.New("send request failed: %w", err)
	}
	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, Error.New("send request failed: %w", err)
	}

	var tokenData struct {
		ExpiresIn   int    `json:"expires_in"`
		AccessToken string `json:"access_token"`
	}
	err = json.NewDecoder(resp.Body).Decode(&tokenData)
	if err != nil {
		return nil, Error.New("decode response failed: %w", err)
	}
	return &TokenData{
		AccessToken: tokenData.AccessToken,
		ExpiresAt:   time.Now().Add(time.Duration(tokenData.ExpiresIn * 1000)),
	}, nil
}
