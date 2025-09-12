// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
)

var (
	// ErrUsageLimitsAPI - console usage and limits api error type.
	ErrUsageLimitsAPI = errs.Class("console usage and limits")
)

// UsageLimits is an api controller that exposes all usage and limits related functionality.
type UsageLimits struct {
	log                    *zap.Logger
	service                *console.Service
	allowedReportDateRange time.Duration
}

// NewUsageLimits is a constructor for api usage and limits controller.
func NewUsageLimits(log *zap.Logger, service *console.Service, allowedReportDateRange time.Duration) *UsageLimits {
	return &UsageLimits{
		log:                    log,
		service:                service,
		allowedReportDateRange: allowedReportDateRange,
	}
}

// ProjectUsageLimits returns usage and limits by project ID.
func (ul *UsageLimits) ProjectUsageLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid project id: %v", err))
		return
	}

	usageLimits, err := ul.service.GetProjectUsageLimits(ctx, projectID)
	if err != nil {
		switch {
		case console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err):
			ul.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		case accounting.ErrInvalidArgument.Has(err):
			ul.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		default:
			ul.serveJSONError(ctx, w, http.StatusInternalServerError, err)
			return
		}
	}

	err = json.NewEncoder(w).Encode(usageLimits)
	if err != nil {
		ul.log.Error("error encoding project usage limits", zap.Error(ErrUsageLimitsAPI.Wrap(err)))
	}
}

// TotalUsageLimits returns total usage and limits for all the projects that user owns.
func (ul *UsageLimits) TotalUsageLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	usageLimits, err := ul.service.GetTotalUsageLimits(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			ul.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		ul.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(usageLimits)
	if err != nil {
		ul.log.Error("error encoding project usage limits", zap.Error(ErrUsageLimitsAPI.Wrap(err)))
	}
}

// UsageReport returns usage report for all the projects that user owns or a single user's project.
func (ul *UsageLimits) UsageReport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	sinceStamp, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}
	beforeStamp, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	since := time.Unix(sinceStamp, 0).UTC()
	before := time.Unix(beforeStamp, 0).UTC()

	duration := before.Sub(since)
	if duration > ul.allowedReportDateRange {
		ul.serveJSONError(ctx, w, http.StatusForbidden, errs.New("date range must be less than %v", ul.allowedReportDateRange))
		return
	}

	var projectID uuid.UUID

	idParam := r.URL.Query().Get("projectID")
	if idParam != "" {
		projectID, err = uuid.FromString(idParam)
		if err != nil {
			ul.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid project id: %v", err))
			return
		}
	}

	includeCost := r.URL.Query().Get("cost") == "true"
	groupByProject := r.URL.Query().Get("project-summary") == "true"

	param := console.GetUsageReportParam{
		ProjectID:      projectID,
		Since:          since,
		Before:         before,
		IncludeCost:    includeCost,
		GroupByProject: groupByProject,
	}
	usage, err := ul.service.GetUsageReport(ctx, param)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			ul.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		ul.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	dateFormat := "2006-01-02"
	fileName := "storj-report-" + idParam + "-" + since.Format(dateFormat) + "-to-" + before.Format(dateFormat) + ".csv"

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment;filename="+fileName)
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	wr := csv.NewWriter(w)

	disclaimerRow, csvHeaders := ul.service.GetUsageReportHeaders(param)

	if len(disclaimerRow) > 0 {
		err = wr.Write(disclaimerRow)
		if err != nil {
			ul.serveJSONError(ctx, w, http.StatusInternalServerError, errs.New("Error writing CSV data"))
			return
		}
	}
	err = wr.Write(csvHeaders)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusInternalServerError, errs.New("Error writing CSV data"))
		return
	}

	for _, u := range usage {
		err = wr.Write(ul.service.GetReportRow(param, u))
		if err != nil {
			ul.serveJSONError(ctx, w, http.StatusInternalServerError, errs.New("Error writing CSV data"))
			return
		}
	}

	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache the same request for 1 hour.

	wr.Flush()
}

// DailyUsage returns daily usage by project ID.
func (ul *UsageLimits) DailyUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	projectID, err := uuid.FromString(idParam)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid project id: %v", err))
		return
	}

	sinceStamp, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}
	beforeStamp, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
	if err != nil {
		ul.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	since := time.Unix(sinceStamp, 0)
	before := time.Unix(beforeStamp, 0)

	dailyUsage, err := ul.service.GetDailyProjectUsage(ctx, projectID, since, before)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			ul.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		ul.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Cache-Control", "public, max-age=3600") // Cache the same request for 1 hour.

	err = json.NewEncoder(w).Encode(dailyUsage)
	if err != nil {
		ul.log.Error("error encoding daily project usage", zap.Error(ErrUsageLimitsAPI.Wrap(err)))
	}
}

// serveJSONError writes JSON error to response output stream.
func (ul *UsageLimits) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, ul.log, w, status, err)
}
