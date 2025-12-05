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
	"strings"
	"sync"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
)

const (
	expiryBufferTime = 5 * time.Minute
)

// HubSpotConfig is a configuration struct for Concurrent Sending of Events.
type HubSpotConfig struct {
	RefreshToken                   string        `help:"hubspot refresh token" default:""`
	TokenAPI                       string        `help:"hubspot token refresh API" default:"https://api.hubapi.com/oauth/v1/token"`
	ClientID                       string        `help:"hubspot client ID" default:""`
	ClientSecret                   string        `help:"hubspot client secret" default:""`
	ChannelSize                    int           `help:"the number of events that can be in the queue before dropping" default:"1000"`
	ConcurrentSends                int           `help:"the number of concurrent api requests that can be made" default:"4"`
	DefaultTimeout                 time.Duration `help:"the default timeout for the hubspot http client" default:"10s"`
	SignupEventName                string        `help:"the event name for signup action" default:""`
	SignupFormURL                  string        `help:"the hubspot form URL for signup" default:""`
	CunoFSBetaFormURL              string        `help:"the hubspot form URL for cunoFS beta" default:""`
	ObjectMountConsultationFormURL string        `help:"the hubspot form URL for requesting object mount consultation" default:""`
	LifeCycleStage                 string        `help:"the hubspot lifecycle stage for new accounts" default:""`

	AccountObjectCreatedWebhookEnabled  bool          `help:"whether account object created webhook is enabled" default:"false"`
	AccountObjectCreatedWebhookEndpoint string        `help:"the endpoint for account object created webhook" default:"/api/v0/analytics/hubspot/account-object-created"`
	WebhookRequestLifetime              time.Duration `help:"the lifetime of the webhook request" default:"5m"`
}

// AccountObjectCreatedRequest is a configuration struct for receiving Account Object Created request from HubSpot.
type AccountObjectCreatedRequest struct {
	UserID   string      `json:"userid"`
	ObjectID json.Number `json:"hs_object_id"`
}

// HubSpotEvent is a configuration struct for sending API request to HubSpot.
type HubSpotEvent struct {
	Data     map[string]interface{}
	Endpoint string
	Method   *string
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

// EnqueueCreateUserMinimal is for creating user in HubSpot using the minimal form.
func (q *HubSpotEvents) EnqueueCreateUserMinimal(fields TrackCreateUserFields) {
	if q.config.SignupFormURL == "" || q.config.SignupEventName == "" {
		q.log.Warn("hubspot signup form URL or event name is not set")
		return
	}

	newField := func(name string, value interface{}) map[string]interface{} {
		return map[string]interface{}{
			"name":  name,
			"value": value,
		}
	}

	formFields := []map[string]interface{}{
		newField("email", fields.Email),
		newField("origin_header", fields.OriginHeader),
		newField("signup_referrer", fields.Referrer),
		newField("account_created", "true"),
		newField("signup_partner", fields.UserAgent),
		newField("lifecyclestage", q.config.LifeCycleStage),
	}
	if fields.SignupCaptcha != nil {
		formFields = append(formFields, newField("signup_captcha_score", *fields.SignupCaptcha))
	}

	properties := map[string]interface{}{
		"userid":             fields.ID.String(),
		"email":              fields.Email,
		"satellite_selected": q.satelliteName,
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
		Endpoint: q.config.SignupFormURL,
		Data:     data,
	}

	sendUserEvent := HubSpotEvent{
		Endpoint: "https://api.hubapi.com/events/v3/send",
		Data: map[string]interface{}{
			"email":      fields.Email,
			"eventName":  q.config.SignupEventName,
			"properties": properties,
		},
	}

	select {
	case q.events <- []HubSpotEvent{createUser, sendUserEvent}:
	default:
		q.log.Error("create user hubspot event failed, event channel is full")
	}
}

// EnqueueJoinCunoFSBeta is for tracking user join cunoFS beta using hubspot form.
func (q *HubSpotEvents) EnqueueJoinCunoFSBeta(fields TrackJoinCunoFSBetaFields) {
	if q.config.CunoFSBetaFormURL == "" {
		q.log.Warn("hubspot cunoFS beta form URL is not set")
		return
	}

	newField := func(name string, value string) map[string]string {
		return map[string]string{
			"name":  name,
			"value": value,
		}
	}

	formFields := []map[string]string{
		newField("email", fields.Email),
		newField("company", fields.CompanyName),
		newField("firstname", fields.FirstName),
		newField("lastname", fields.LastName),
		newField("industry_use_case", fields.IndustryUseCase),
		newField("other_industry_use_case", fields.OtherIndustryUseCase),
		newField("operating_system", fields.OperatingSystem),
		newField("team_size", fields.TeamSize),
		newField("current_storage_usage", fields.CurrentStorageUsage),
		newField("infrastructure_type", fields.InfraType),
		newField("current_storage_backends", fields.CurrentStorageBackends),
		newField("other_storage_backend", fields.OtherStorageBackend),
		newField("current_storage_mount_solution", fields.CurrentStorageMountSolution),
		newField("other_storage_mount_solution", fields.OtherStorageMountSolution),
		newField("desired_features", fields.DesiredFeatures),
		newField("current_pain_points", fields.CurrentPainPoints),
		newField("cunofs_beta_specific_tasks", fields.SpecificTasks),
	}

	joinBetaEvent := HubSpotEvent{
		Endpoint: q.config.CunoFSBetaFormURL,
		Data: map[string]interface{}{
			"fields": formFields,
		},
	}

	select {
	case q.events <- []HubSpotEvent{joinBetaEvent}:
	default:
		q.log.Error("join cunoFS beta hubspot event failed, event channel is full")
	}
}

// EnqueueJoinPlacementWaitlist is for tracking user joining placement waitlist using hubspot form.
func (q *HubSpotEvents) EnqueueJoinPlacementWaitlist(fields TrackJoinPlacementWaitlistFields) {
	if fields.WaitlistURL == "" {
		q.log.Warn("hubspot placement waitlist form URL is not set")
		return
	}

	newField := func(name string, value string) map[string]string {
		return map[string]string{
			"name":  name,
			"value": value,
		}
	}

	formFields := []map[string]string{
		newField("email", fields.Email),
		newField("us_select_storage_interest", fields.StorageNeeds),
	}

	joinWaitlistEvent := HubSpotEvent{
		Endpoint: fields.WaitlistURL,
		Data: map[string]interface{}{
			"fields": formFields,
		},
	}

	select {
	case q.events <- []HubSpotEvent{joinWaitlistEvent}:
	default:
		q.log.Error("join placement waitlist hubspot event failed, event channel is full")
	}
}

// EnqueueObjectMountConsultation is for tracking user object mount consultation request using hubspot form.
func (q *HubSpotEvents) EnqueueObjectMountConsultation(fields TrackObjectMountConsultationFields) {
	if q.config.ObjectMountConsultationFormURL == "" {
		q.log.Warn("hubspot object mount consultation form URL is not set")
		return
	}

	newField := func(name string, value string) map[string]string {
		return map[string]string{
			"name":  name,
			"value": value,
		}
	}

	formFields := []map[string]string{
		newField("email", fields.Email),
		newField("company", fields.CompanyName),
		newField("firstname", fields.FirstName),
		newField("lastname", fields.LastName),
		newField("job_title", fields.JobTitle),
		newField("phone", fields.PhoneNumber),
		newField("industry_use_case", fields.IndustryUseCase),
		newField("company_size_dropdown", fields.CompanySize),
		newField("current_storage_solution", fields.CurrentStorageSolution),
		newField("key_challenges", fields.KeyChallenges),
		newField("specific_interests", fields.SpecificInterests),
		newField("storage_needs", fields.StorageNeeds),
		newField("implementation_timeline", fields.ImplementationTimeline),
		newField("additional_information", fields.AdditionalInformation),
	}

	requestObjectMountConsultation := HubSpotEvent{
		Endpoint: q.config.ObjectMountConsultationFormURL,
		Data: map[string]interface{}{
			"fields": formFields,
		},
	}

	select {
	case q.events <- []HubSpotEvent{requestObjectMountConsultation}:
	default:
		q.log.Error("request object mount consultation hubspot event failed, event channel is full")
	}
}

// EnqueueUserOnboardingInfo is for sending post-creation information to Hubspot, that the user enters after login  during onboarding.
func (q *HubSpotEvents) EnqueueUserOnboardingInfo(fields TrackOnboardingInfoFields) {
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

	properties := map[string]interface{}{
		"email":     fields.Email,
		"firstname": firstName,
		"lastname":  lastName,
		"use_case":  fields.StorageUseCase,
	}
	if fields.Type == Professional {
		properties["have_sales_contact"] = fields.HaveSalesContact
		properties["interested_in_partnering_"] = fields.InterestedInPartnering // trailing underscore in property name is not a mistake
		properties["company_size"] = fields.EmployeeCount
		properties["company"] = fields.CompanyName
		properties["jobtitle"] = fields.JobTitle
		properties["storage_needs"] = fields.StorageNeeds
		properties["functional_area"] = fields.FunctionalArea
	}

	updateContactEndpoint := fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/contacts/%s?idProperty=email", url.QueryEscape(fields.Email))
	method := http.MethodPatch

	onboardingInfoEvent := HubSpotEvent{
		Endpoint: updateContactEndpoint,
		Method:   &method,
		Data: map[string]interface{}{
			"properties": properties,
		},
	}

	select {
	case q.events <- []HubSpotEvent{onboardingInfoEvent}:
	default:
		q.log.Error("update user properties hubspot event failed, event channel is full")
	}
}

// EnqueueUserChangeEmail is for sending post-creation information to Hubspot, that the user changed their email.
func (q *HubSpotEvents) EnqueueUserChangeEmail(oldEmail, newEmail string) {
	properties := map[string]interface{}{
		"email": newEmail,
	}

	updateContactEndpoint := fmt.Sprintf("https://api.hubapi.com/crm/v3/objects/contacts/%s?idProperty=email", url.QueryEscape(oldEmail))
	method := http.MethodPatch

	changeEmailEvent := HubSpotEvent{
		Endpoint: updateContactEndpoint,
		Method:   &method,
		Data: map[string]interface{}{
			"properties": properties,
		},
	}

	select {
	case q.events <- []HubSpotEvent{changeEmailEvent}:
	default:
		q.log.Error("update user email hubspot event failed, event channel is full")
	}
}

// handleSingleEvent for handle the single HubSpot API request.
func (q *HubSpotEvents) handleSingleEvent(ctx context.Context, ev HubSpotEvent) (err error) {
	payloadBytes, err := json.Marshal(ev.Data)
	if err != nil {
		return Error.New("json marshal failed: %w", err)
	}

	method := http.MethodPost
	if ev.Method != nil {
		method = *ev.Method
	}
	req, err := http.NewRequestWithContext(ctx, method, ev.Endpoint, bytes.NewReader(payloadBytes))
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
		q.log.Error("send request failed", zap.Error(err))
		return Error.New("send request failed: %w", err)
	}

	defer func() {
		err = errs.Combine(err, resp.Body.Close())
	}()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		type hsError struct {
			Message string `json:"message"`
			In      string `json:"in"`
		}
		var data struct {
			Message string    `json:"message"`
			Errors  []hsError `json:"errors"`
		}
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			return Error.New("decoding response failed: %w", err)
		}
		return Error.New("sending event failed: %s - %v", data.Message, data.Errors)
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
