// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/zeebo/errs/v2"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/stripe"
)

func (server *Server) checkProjectUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if !server.checkUsage(ctx, w, projectUUID) {
		sendJSONData(w, http.StatusOK, []byte(`{"result":"no project usage exist"}`))
	}
}

func (server *Server) getProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if err != nil {
		sendJSONError(w, "unable to fetch project details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(project)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) getProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var output struct {
		Usage struct {
			Amount memory.Size `json:"amount"`
			Bytes  int64       `json:"bytes"`
		} `json:"usage"`
		Bandwidth struct {
			Amount memory.Size `json:"amount"`
			Bytes  int64       `json:"bytes"`
		} `json:"bandwidth"`
		Rate struct {
			RPS int `json:"rps"`
		} `json:"rate"`
		Buckets  int   `json:"maxBuckets"`
		Segments int64 `json:"maxSegments"`
	}
	if project.StorageLimit != nil {
		output.Usage.Amount = *project.StorageLimit
		output.Usage.Bytes = project.StorageLimit.Int64()
	}
	if project.BandwidthLimit != nil {
		output.Bandwidth.Amount = *project.BandwidthLimit
		output.Bandwidth.Bytes = project.BandwidthLimit.Int64()
	}
	if project.MaxBuckets != nil {
		output.Buckets = *project.MaxBuckets
	}
	if project.RateLimit != nil {
		output.Rate.RPS = *project.RateLimit
	}
	if project.SegmentLimit != nil {
		output.Segments = *project.SegmentLimit
	}

	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) putProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	var arguments struct {
		Usage     *memory.Size `schema:"usage"`
		Bandwidth *memory.Size `schema:"bandwidth"`
		Rate      *int         `schema:"rate"`
		Burst     *int         `schema:"burst"`
		Buckets   *int         `schema:"buckets"`
		Segments  *int64       `schema:"segments"`
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	decoder := schema.NewDecoder()
	err = decoder.Decode(&arguments, r.Form)
	if err != nil {
		sendJSONError(w, "invalid arguments",
			err.Error(), http.StatusBadRequest)
		return
	}

	// check if the project exists.
	_, err = server.db.Console().Projects().Get(ctx, projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	if arguments.Usage != nil {
		if *arguments.Usage < 0 {
			sendJSONError(w, "negative usage",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectUsageLimit(ctx, projectUUID, *arguments.Usage)
		if err != nil {
			sendJSONError(w, "failed to update usage",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Bandwidth != nil {
		if *arguments.Bandwidth < 0 {
			sendJSONError(w, "negative bandwidth",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, projectUUID, *arguments.Bandwidth)
		if err != nil {
			sendJSONError(w, "failed to update bandwidth",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Rate != nil {
		if *arguments.Rate < 0 {
			sendJSONError(w, "negative rate",
				fmt.Sprintf("%v", arguments.Rate), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateRateLimit(ctx, projectUUID, *arguments.Rate)
		if err != nil {
			sendJSONError(w, "failed to update rate",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Burst != nil {
		if *arguments.Burst < 0 {
			sendJSONError(w, "negative burst rate",
				fmt.Sprintf("%v", arguments.Burst), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateBurstLimit(ctx, projectUUID, *arguments.Burst)
		if err != nil {
			sendJSONError(w, "failed to update burst",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Buckets != nil {
		if *arguments.Buckets < 0 {
			sendJSONError(w, "negative bucket count",
				fmt.Sprintf("t: %v", arguments.Buckets), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateBucketLimit(ctx, projectUUID, *arguments.Buckets)
		if err != nil {
			sendJSONError(w, "failed to update bucket limit",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Segments != nil {
		if *arguments.Segments < 0 {
			sendJSONError(w, "negative segments count",
				fmt.Sprintf("t: %v", arguments.Buckets), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectSegmentLimit(ctx, projectUUID, *arguments.Segments)
		if err != nil {
			sendJSONError(w, "failed to update segments limit",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (server *Server) addProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		OwnerID     uuid.UUID `json:"ownerId"`
		ProjectName string    `json:"projectName"`
	}

	var output struct {
		ProjectID uuid.UUID `json:"projectId"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.OwnerID.IsZero() {
		sendJSONError(w, "OwnerID is not set",
			"", http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		sendJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Insert(ctx, &console.Project{
		Name:    input.ProjectName,
		OwnerID: input.OwnerID,
	})
	if err != nil {
		sendJSONError(w, "failed to insert project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = server.db.Console().ProjectMembers().Insert(ctx, project.OwnerID, project.ID)
	if err != nil {
		sendJSONError(w, "failed to insert project member",
			err.Error(), http.StatusInternalServerError)
		return
	}

	output.ProjectID = project.ID
	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) renameProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "ailed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		ProjectName string `json:"projectName"`
		Description string `json:"description"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		sendJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	project.Name = input.ProjectName
	if input.Description != "" {
		project.Description = input.Description
	}

	err = server.db.Console().Projects().Update(ctx, project)
	if err != nil {
		sendJSONError(w, "error renaming project",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) updateProjectsUserAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, "project with specified uuid does not exist",
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	creationDatePlusMonth := project.CreatedAt.AddDate(0, 1, 0)
	if time.Now().After(creationDatePlusMonth) {
		sendJSONError(w, "this project was created more than a month ago",
			"we should update user agent only for recently created projects", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		UserAgent string `json:"userAgent"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.UserAgent == "" {
		sendJSONError(w, "UserAgent was not provided",
			"", http.StatusBadRequest)
		return
	}

	newUserAgent := []byte(input.UserAgent)

	if bytes.Equal(project.UserAgent, newUserAgent) {
		sendJSONError(w, "new UserAgent is equal to existing projects UserAgent",
			"", http.StatusBadRequest)
		return
	}

	err = server._updateProjectsUserAgent(ctx, project.ID, newUserAgent)
	if err != nil {
		sendJSONError(w, "failed to update projects user agent",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		sendJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		sendJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		sendJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	options := buckets.ListOptions{Limit: 1, Direction: buckets.DirectionForward}
	buckets, err := server.buckets.ListBuckets(ctx, projectUUID, options, macaroon.AllowedBuckets{All: true})
	if err != nil {
		sendJSONError(w, "unable to list buckets",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(buckets.Items) > 0 {
		sendJSONError(w, "buckets still exist",
			fmt.Sprintf("%v", bucketNames(buckets.Items)), http.StatusConflict)
		return
	}

	keys, err := server.db.Console().APIKeys().GetPagedByProjectID(ctx, projectUUID, console.APIKeyCursor{Limit: 1, Page: 1})
	if err != nil {
		sendJSONError(w, "unable to list api-keys",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if keys.TotalCount > 0 {
		sendJSONError(w, "api-keys still exist",
			fmt.Sprintf("count %d", keys.TotalCount), http.StatusConflict)
		return
	}

	// if usage exist, return error to client and exit
	if server.checkUsage(ctx, w, projectUUID) {
		return
	}

	err = server.db.Console().Projects().Delete(ctx, projectUUID)
	if err != nil {
		sendJSONError(w, "unable to delete project",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) _updateProjectsUserAgent(ctx context.Context, projectID uuid.UUID, newUserAgent []byte) (err error) {
	err = server.db.Console().Projects().UpdateUserAgent(ctx, projectID, newUserAgent)
	if err != nil {
		return err
	}

	listOptions := buckets.ListOptions{
		Direction: buckets.DirectionForward,
	}

	allowedBuckets := macaroon.AllowedBuckets{
		All: true,
	}

	projectBuckets, err := server.db.Buckets().ListBuckets(ctx, projectID, listOptions, allowedBuckets)
	if err != nil {
		return err
	}

	var errList errs.Group
	for _, bucket := range projectBuckets.Items {
		err = server.db.Buckets().UpdateUserAgent(ctx, projectID, bucket.Name, newUserAgent)
		if err != nil {
			errList.Append(err)
		}

		err = server.db.Attribution().UpdateUserAgent(ctx, projectID, bucket.Name, newUserAgent)
		if err != nil {
			errList.Append(err)
		}
	}

	if errList.Err() != nil {
		return errList.Err()
	}

	return nil
}

func (server *Server) checkInvoicing(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID) (openInvoices bool) {
	year, month, _ := server.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	// Check if an invoice project record exists already
	err := server.db.StripeCoinPayments().ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
	if errors.Is(err, stripe.ErrProjectRecordExists) {
		record, err := server.db.StripeCoinPayments().ProjectRecords().Get(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
		if err != nil {
			sendJSONError(w, "unable to get project records", err.Error(), http.StatusInternalServerError)
			return true
		}
		// state = 0 means unapplied and not invoiced yet.
		if record.State == 0 {
			sendJSONError(w, "unapplied project invoice record exist", "", http.StatusConflict)
			return true
		}
		// Record has been applied, so project can be deleted.
		return false
	}
	if err != nil {
		sendJSONError(w, "unable to get project records", err.Error(), http.StatusInternalServerError)
		return true
	}

	return false
}

func (server *Server) checkUsage(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID) (hasUsage bool) {
	year, month, _ := server.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	prj, err := server.db.Console().Projects().Get(ctx, projectID)
	if err != nil {
		sendJSONError(w, "unable to get project details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	// If user is paid tier, check the usage limit, otherwise it is ok to delete it.
	paid, err := server.db.Console().Users().GetUserPaidTier(ctx, prj.OwnerID)
	if err != nil {
		sendJSONError(w, "unable to project owner tier",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if paid {
		// check current month usage and do not allow deletion if usage exists
		currentUsage, err := server.db.ProjectAccounting().GetProjectTotal(ctx, projectID, firstOfMonth, server.nowFn())
		if err != nil {
			sendJSONError(w, "unable to list project usage", err.Error(), http.StatusInternalServerError)
			return true
		}
		if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.SegmentCount > 0 {
			sendJSONError(w, "usage for current month exists", "", http.StatusConflict)
			return true
		}

		// check usage for last month, if exists, ensure we have an invoice item created.
		lastMonthUsage, err := server.db.ProjectAccounting().GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
		if err != nil {
			sendJSONError(w, "error getting project totals",
				"", http.StatusInternalServerError)
			return true
		}
		if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.SegmentCount > 0 {
			err = server.db.StripeCoinPayments().ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth)
			if !errors.Is(err, stripe.ErrProjectRecordExists) {
				sendJSONError(w, "usage for last month exist, but is not billed yet", "", http.StatusConflict)
				return true
			}
		}
	}

	// If we have open invoice items, do not delete the project yet and wait for invoice completion.
	return server.checkInvoicing(ctx, w, projectID)
}

func bucketNames(buckets []buckets.Bucket) []string {
	var xs []string
	for _, b := range buckets {
		xs = append(xs, b.Name)
	}
	return xs
}
