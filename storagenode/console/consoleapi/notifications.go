// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/storagenode/notifications"
)

// ErrNotificationsAPI - console notifications api error type.
var ErrNotificationsAPI = errs.Class("consoleapi notifications")

// Notifications is an api controller that exposes all notifications related api.
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

// NewNotifications is a constructor for notifications controller.
func NewNotifications(log *zap.Logger, service *notifications.Service) *Notifications {
	return &Notifications{
		log:     log,
		service: service,
	}
}

// ReadNotification updates specific notification in database as read.
func (notification *Notifications) ReadNotification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	params := mux.Vars(r)
	id, ok := params["id"]
	if !ok {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
		return
	}

	notificationID, err := uuid.FromString(id)
	if err != nil {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
		return
	}

	err = notification.service.Read(ctx, notificationID)
	if err != nil {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
		return
	}
}

// ReadAllNotifications updates all notifications in database as read.
func (notification *Notifications) ReadAllNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	err = notification.service.ReadAll(ctx)
	if err != nil {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
		return
	}
}

// ListNotifications returns listed page of notifications from database.
func (notification *Notifications) ListNotifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set(contentType, applicationJSON)

	limit, err := strconv.ParseUint(r.URL.Query().Get("limit"), 10, 32)
	if err != nil {
		notification.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	page, err := strconv.ParseUint(r.URL.Query().Get("page"), 10, 32)
	if err != nil {
		notification.serveJSONError(w, http.StatusBadRequest, ErrNotificationsAPI.Wrap(err))
		return
	}

	cursor := notifications.Cursor{
		Limit: uint(limit),
		Page:  uint(page),
	}

	notificationList, err := notification.service.List(ctx, cursor)
	if err != nil {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
		return
	}

	unreadCount, err := notification.service.UnreadAmount(ctx)
	if err != nil {
		notification.serveJSONError(w, http.StatusInternalServerError, ErrNotificationsAPI.Wrap(err))
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

	if err := json.NewEncoder(w).Encode(result); err != nil {
		notification.log.Error("failed to encode json list notifications response", zap.Error(ErrNotificationsAPI.Wrap(err)))
		return
	}
}

// serveJSONError writes JSON error to response output stream.
func (notification *Notifications) serveJSONError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)

	var response struct {
		Error string `json:"error"`
	}

	response.Error = err.Error()

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		notification.log.Error("failed to write json error response", zap.Error(ErrNotificationsAPI.Wrap(err)))
		return
	}
}
