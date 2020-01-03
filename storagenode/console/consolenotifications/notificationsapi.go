// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consolenotifications

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/storagenode/notifications"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

var mon = monkit.Package()

// Error is error type of storagenode web console.
var Error = errs.Class("notifications console web error")

// Notifications represents notification service.
// architecture: Service
type Notifications struct {
	service *notifications.Service

	log *zap.Logger
}

// jsonOutput defines json structure of api response data.
type jsonOutput struct {
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}

// NewNotifications creates new instance of notification service.
func NewNotifications(log *zap.Logger, service *notifications.Service) *Notifications {
	return &Notifications{
		log:     log,
		service: service,
	}
}

// ReadNotification updates specific notification in database as read.
func (notification *Notifications) ReadNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	var err error

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	notificationID, err := uuid.Parse(id)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	err = notification.service.Read(ctx, *notificationID)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}
}

// ReadAllNotifications updates all notifications in database as read.
func (notification *Notifications) ReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	err := notification.service.ReadAll(ctx)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}
}

// ListNotifications returns listed page of notifications from database.
func (notification *Notifications) ListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	var err error

	var request struct {
		Cursor notifications.Cursor `json:"cursor"`
	}

	err = json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	page, err := notification.service.List(ctx, request.Cursor)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	notification.writeData(w, page)
}

// writeData is helper method to write JSON to http.ResponseWriter and log encoding error.
func (notification *Notifications) writeData(w http.ResponseWriter, data interface{}) {
	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(http.StatusOK)

	output := jsonOutput{Data: data}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		notification.log.Error("json encoder error", zap.Error(err))
	}
}

// writeError writes a JSON error payload to http.ResponseWriter log encoding error.
func (notification *Notifications) writeError(w http.ResponseWriter, status int, err error) {
	if status >= http.StatusInternalServerError {
		notification.log.Error("api handler server error", zap.Int("status code", status), zap.Error(err))
	}

	w.Header().Set(contentType, applicationJSON)
	w.WriteHeader(status)

	output := jsonOutput{Error: err.Error()}

	if err := json.NewEncoder(w).Encode(output); err != nil {
		notification.log.Error("json encoder error", zap.Error(err))
	}
}
