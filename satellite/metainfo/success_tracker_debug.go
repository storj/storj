// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/debug"
	"storj.io/common/storj"
	"storj.io/storj/satellite/nodeselection"
	"storj.io/storj/satellite/overlay"
)

// TrackerInfo is an interface that provides information about the current trackers.
type TrackerInfo struct {
	db                overlay.DB
	trackers          *Trackers
	prometheusTracker nodeselection.ScoreNode
}

// NewTrackerInfo creates a new TrackerInfo.
func NewTrackerInfo(trackers *Trackers, db overlay.DB) *TrackerInfo {
	t := &TrackerInfo{
		db:       db,
		trackers: trackers,
	}
	return t
}

// WithPrometheusTracker adds the prometheus tracker to the TrackerInfo.
func (t *TrackerInfo) WithPrometheusTracker(prometheusTracker nodeselection.ScoreNode) *TrackerInfo {
	t.prometheusTracker = prometheusTracker
	return t
}

var _ debug.Extension = &TrackerInfo{}

// Description implements the debug.Extension interface.
func (t *TrackerInfo) Description() string {
	return "Information about the current state of the trackers"
}

// Path implements the debug.Extension interface.
func (t *TrackerInfo) Path() string {
	return "/trackers"
}

// Handler returns a http.HandlerFunc that serves the tracker information.
func (t *TrackerInfo) Handler(writer http.ResponseWriter, request *http.Request) {
	success := request.URL.Query().Get("success")
	failure := request.URL.Query().Get("failure")
	prometheus := request.URL.Query().Get("prometheus")
	switch {
	case success != "":
		result, err := t.DumpSuccessTracker(request.Context(), success)
		asText(writer, result, err)
	case failure != "":
		result, err := t.DumpTracker(request.Context(), t.trackers.GetFailureTracker())
		asText(writer, result, err)
	case request.URL.Query().Get("retry") != "":
		result, err := t.DumpTracker(request.Context(), t.trackers.GetRetryTracker())
		asText(writer, result, err)
	case prometheus != "":
		if t.prometheusTracker == nil {
			asText(writer, "", fmt.Errorf("prometheus tracker not configured"))
			return
		}
		result, err := t.DumpPrometheusTracker(request.Context())
		asText(writer, result, err)
	default:
		result, err := t.ListTrackers(request.Context())
		asText(writer, result, err)
	}
}

// TrackerInfoExtension is a debug.Extension which serves the TrackerInfo once
// it is set. The debug server needs its extension list before the trackers are
// initialized, so this placeholder is registered instead of TrackerInfo itself.
type TrackerInfoExtension struct {
	info atomic.Pointer[TrackerInfo]
}

var _ debug.Extension = &TrackerInfoExtension{}

// Set sets the TrackerInfo to serve.
func (e *TrackerInfoExtension) Set(info *TrackerInfo) {
	e.info.Store(info)
}

// Description implements the debug.Extension interface.
func (e *TrackerInfoExtension) Description() string {
	return "Information about the current state of the trackers (if available)"
}

// Path implements the debug.Extension interface.
func (e *TrackerInfoExtension) Path() string {
	return "/trackers"
}

// Handler implements the debug.Extension interface.
func (e *TrackerInfoExtension) Handler(writer http.ResponseWriter, request *http.Request) {
	info := e.info.Load()
	if info == nil {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("success trackers are not available in this process"))
		return
	}
	info.Handler(writer, request)
}

// asText writes the result as text to the HTTP writer.
func asText(writer http.ResponseWriter, result string, err error) {
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}
	writer.WriteHeader(http.StatusOK)
	writer.Header().Set("Content-Type", "text/plain")
	_, _ = writer.Write([]byte(result))
}

// DumpSuccessTracker prints out all scores for one specific tracker..
func (t *TrackerInfo) DumpSuccessTracker(ctx context.Context, success string) (string, error) {
	var uplink storj.NodeID
	if success != "global" && success != "" {
		url, err := storj.NodeIDFromString(success)
		if err != nil {
			return "", errs.Wrap(err)
		}
		uplink = url
	}
	tracker := t.trackers.GetTracker(uplink)
	return t.DumpTracker(ctx, tracker)
}

// DumpTracker prints out all scores for all nodes.
func (t *TrackerInfo) DumpTracker(ctx context.Context, tracker SuccessTracker) (string, error) {
	nodes, err := t.db.GetAllParticipatingNodes(ctx, 4*time.Hour, -1*time.Microsecond)
	if err != nil {
		return "", errs.Wrap(err)
	}
	output := ""
	for _, node := range nodes {
		value := tracker.Get(&node)
		output += fmt.Sprintf("%s %f\n", node.ID, value)
	}
	return output, nil
}

// DumpPrometheusTracker prints out all scores from the prometheus tracker.
func (t *TrackerInfo) DumpPrometheusTracker(ctx context.Context) (string, error) {
	nodes, err := t.db.GetAllParticipatingNodes(ctx, 4*time.Hour, -1*time.Microsecond)
	if err != nil {
		return "", errs.Wrap(err)
	}
	output := ""
	scoreFunc := t.prometheusTracker.Get(storj.NodeID{})
	for _, node := range nodes {
		value := scoreFunc(&node)
		output += fmt.Sprintf("%s %f\n", node.ID, value)
	}
	return output, nil
}

// ListTrackers prints out all available trackers.
func (t *TrackerInfo) ListTrackers(ctx context.Context) (out string, err error) {
	out += "/trackers?failure=true\n"
	out += "/trackers?retry=true\n"
	out += "/trackers?success=global\n"
	for _, uplink := range t.trackers.DedicatedUplinks() {
		out += fmt.Sprintf("/trackers?success=%s\n", uplink)
	}
	if t.prometheusTracker != nil {
		out += "/trackers?prometheus=true\n"
	}
	return out, err
}
