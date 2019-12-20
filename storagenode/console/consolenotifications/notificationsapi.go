// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consolenotifications

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

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

// Page contains notifications and related information.
type Page struct {
	Notifications []notifications.Notification `json:"notifications"`

	Offset      uint64 `json:"offset"`
	Limit       uint   `json:"limit"`
	CurrentPage uint   `json:"currentPage"`
	PageCount   uint   `json:"pageCount"`
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

	limit, err := strconv.ParseUint(r.URL.Query().Get("limit"), 10, 32)
	if err != nil {
		notification.writeError(w, http.StatusBadRequest, Error.Wrap(err))
		return
	}

	page, err := strconv.ParseUint(r.URL.Query().Get("page"), 10, 32)
	if err != nil {
		notification.writeError(w, http.StatusBadRequest, Error.Wrap(err))
		return
	}

	cursor := notifications.Cursor{
		Limit: uint(limit),
		Page:  uint(page),
	}

	notificationList, err := notification.service.List(ctx, cursor)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	unreadCount, err := notification.service.UnreadAmount(ctx)
	if err != nil {
		notification.writeError(w, http.StatusInternalServerError, Error.Wrap(err))
		return
	}

	var result struct {
		Page        Page `json:"page"`
		UnreadCount int  `json:"unreadCount"`
		TotalCount  int  `json:"totalCount"`
	}

	result.Page.Notifications = notificationList.Notifications
	result.Page.Limit = notificationList.Limit
	result.Page.CurrentPage = notificationList.CurrentPage
	result.Page.Offset = notificationList.Offset
	result.Page.PageCount = notificationList.PageCount
	result.UnreadCount = unreadCount
	result.TotalCount = int(notificationList.TotalCount)

	notification.writeData(w, result)
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
