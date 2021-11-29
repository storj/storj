// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
)

var (
	// ErrAnalyticsAPI - console analytics api error type.
	ErrAnalyticsAPI = errs.Class("consoleapi analytics")
)

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
	EventName string `json:"eventName"`
	Link      string `json:"link"`
}

// EventTriggered tracks the occurrence of an arbitrary event on the client.
func (a *Analytics) EventTriggered(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
	}
	var et eventTriggeredBody
	err = json.Unmarshal(body, &et)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
	}

	auth, err := console.GetAuth(ctx)
	if err != nil {
		a.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}
	if et.Link != "" {
		a.analytics.TrackLinkEvent(et.EventName, auth.User.ID, auth.User.Email, et.Link)
	} else {
		a.analytics.TrackEvent(et.EventName, auth.User.ID, auth.User.Email)
	}
	w.WriteHeader(http.StatusOK)
}

// serveJSONError writes JSON error to response output stream.
func (a *Analytics) serveJSONError(w http.ResponseWriter, status int, err error) {
	serveJSONError(a.log, w, status, err)
}
