// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
)

// ErrAnalyticsAPI - console analytics api error type.
var ErrAnalyticsAPI = errs.Class("consoleapi analytics")

// Analytics is an api controller that exposes analytics related functionality.
type Analytics struct {
	log       *zap.Logger
	service   *console.Service
	analytics *analytics.Service
}

// NewAnalytics is a constructor for api analytics controller.
func NewAnalytics(log *zap.Logger, service *console.Service, a *analytics.Service) *Analytics {
	return &Analytics{
		log:       log,
		service:   service,
		analytics: a,
	}
}

type eventTriggeredBody struct {
	EventName            string            `json:"eventName"`
	Link                 string            `json:"link"`
	ErrorEventSource     string            `json:"errorEventSource"`
	ErrorEventRequestID  string            `json:"errorEventRequestID"`
	ErrorEventStatusCode int               `json:"errorEventStatusCode"`
	Props                map[string]string `json:"props"`
}

type pageVisitBody struct {
	PageName string `json:"pageName"`
}

// EventTriggered tracks the occurrence of an arbitrary event on the client.
func (a *Analytics) EventTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
	var et eventTriggeredBody
	err = json.Unmarshal(body, &et)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	user, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	if et.ErrorEventSource != "" {
		a.analytics.TrackErrorEvent(user.ID, user.Email, et.ErrorEventSource, et.ErrorEventRequestID, et.ErrorEventStatusCode, user.HubspotObjectID, user.TenantID)
	} else if et.Link != "" {
		a.analytics.TrackLinkEvent(et.EventName, user.ID, user.Email, et.Link, user.HubspotObjectID, user.TenantID)
	} else {
		a.analytics.TrackEvent(et.EventName, user.ID, user.Email, et.Props, user.HubspotObjectID, user.TenantID)
	}
	w.WriteHeader(http.StatusOK)
}

// PageEventTriggered tracks the occurrence of an arbitrary page visit event on the client.
func (a *Analytics) PageEventTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
	var pv pageVisitBody
	err = json.Unmarshal(body, &pv)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	user, err := console.GetUser(ctx)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	a.analytics.PageVisitEvent(pv.PageName, user.ID, user.Email, user.HubspotObjectID, user.TenantID)

	w.WriteHeader(http.StatusOK)
}

// PageViewTriggered sends a pageview event to plausible.
func (a *Analytics) PageViewTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var pv analytics.PageViewBody
	err = json.NewDecoder(r.Body).Decode(&pv)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	pv.IP, err = web.GetRequestIP(r)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
	pv.UserAgent = r.UserAgent()

	err = a.analytics.PageViewEvent(ctx, pv)
	if err != nil {
		a.log.Error("failed to send pageview event to plausible", zap.Error(err))
	}
	w.WriteHeader(http.StatusAccepted)
}

// JoinCunoFSBeta sends a join form data event to hubspot.
func (a *Analytics) JoinCunoFSBeta(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data analytics.TrackJoinCunoFSBetaFields
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = a.service.ValidateFreeFormFieldLengths(
		&data.FirstName, &data.LastName,
		&data.CompanyName, &data.IndustryUseCase,
		&data.OtherIndustryUseCase, &data.OtherStorageBackend,
		&data.OtherStorageMountSolution,
	); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if err = a.service.ValidateLongFormInputLengths(&data.SpecificTasks); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("specific tasks field is too long"))
		return
	}

	err = a.service.JoinCunoFSBeta(ctx, data)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrBotUser.Has(err) {
			a.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		if console.ErrConflict.Has(err) {
			a.serveJSONError(ctx, w, http.StatusConflict, err)
			return
		}

		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// SendFeedback sends a user feedback form data event to segment.
func (a *Analytics) SendFeedback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var data analytics.UserFeedbackFormData
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	if err = a.service.ValidateFreeFormFieldLengths(&data.Type); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("feedback type is too long"))
		return
	}
	if err = a.service.ValidateLongFormInputLengths(&data.Message); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("message is too long"))
		return
	}

	err = a.service.SendUserFeedback(ctx, data)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if console.ErrBotUser.Has(err) || console.ErrForbidden.Has(err) {
			a.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// JoinPlacementWaitlist sends a placement waitlist form event to hubspot.
func (a *Analytics) JoinPlacementWaitlist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data analytics.TrackJoinPlacementWaitlistFields
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = a.service.JoinPlacementWaitlist(ctx, data)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrBotUser.Has(err) {
			a.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		if console.ErrConflict.Has(err) {
			a.serveJSONError(ctx, w, http.StatusConflict, err)
			return
		}

		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// RequestObjectMountConsultation sends a consultation form data event to hubspot.
func (a *Analytics) RequestObjectMountConsultation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var data analytics.TrackObjectMountConsultationFields
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	if err = a.service.ValidateFreeFormFieldLengths(
		&data.FirstName, &data.LastName, &data.JobTitle, &data.CompanyName, &data.PhoneNumber,
	); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if err = a.service.ValidateLongFormInputLengths(&data.CurrentStorageSolution, &data.KeyChallenges, &data.AdditionalInformation); err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("long-form input field exceeds max length"))
		return
	}

	err = a.service.RequestObjectMountConsultation(ctx, data)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			a.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrBotUser.Has(err) {
			a.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		if console.ErrConflict.Has(err) {
			a.serveJSONError(ctx, w, http.StatusConflict, err)
			return
		}

		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// AccountObjectCreated handles the webhook from hubspot when an account object is created.
func (a *Analytics) AccountObjectCreated(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	signature := r.Header.Get("x-hubspot-signature-v3")
	if signature == "" {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("missing request signature"))
		return
	}

	timestamp := r.Header.Get("x-hubspot-request-timestamp")
	if timestamp == "" {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("missing request timestamp"))
		return
	}

	var req analytics.AccountObjectCreatedRequest
	err = json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.Wrap(err))
		return
	}

	if req.UserID == "" {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("missing user id"))
		return
	}
	if req.ObjectID == "" {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("missing object id"))
		return
	}

	userID, err := uuid.FromString(req.UserID)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.New("invalid user id"))
		return
	}

	err = a.analytics.ValidateAccountObjectCreatedRequestSignature(req, signature, timestamp)
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusBadRequest, ErrAnalyticsAPI.Wrap(err))
		return
	}

	err = a.service.UpdateUserHubspotObjectID(ctx, userID, req.ObjectID.String())
	if err != nil {
		a.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// serveJSONError writes JSON error to response output stream.
func (a *Analytics) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, a.log, w, status, err)
}
