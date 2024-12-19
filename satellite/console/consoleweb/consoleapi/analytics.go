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
	EventName        string            `json:"eventName"`
	Link             string            `json:"link"`
	ErrorEventSource string            `json:"errorEventSource"`
	Props            map[string]string `json:"props"`
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
		a.analytics.TrackErrorEvent(user.ID, user.Email, et.ErrorEventSource)
	} else if et.Link != "" {
		a.analytics.TrackLinkEvent(et.EventName, user.ID, user.Email, et.Link)
	} else {
		a.analytics.TrackEvent(et.EventName, user.ID, user.Email, et.Props)
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

	a.analytics.PageVisitEvent(pv.PageName, user.ID, user.Email)

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

// serveJSONError writes JSON error to response output stream.
func (a *Analytics) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, a.log, w, status, err)
}
