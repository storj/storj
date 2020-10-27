// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments/stripecoinpayments"
)

func (server *Server) checkProjectUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if !server.checkUsage(ctx, w, projectUUID) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"result":"no project usage exist"}`))
	}
}

func (server *Server) getProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		httpJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if err != nil {
		httpJSONError(w, "unable to fetch project details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(project)
	if err != nil {
		httpJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) getProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	usagelimit, err := server.db.ProjectAccounting().GetProjectStorageLimit(ctx, projectUUID)
	if err != nil {
		httpJSONError(w, "failed to get usage limit",
			err.Error(), http.StatusInternalServerError)
		return
	}

	bandwidthlimit, err := server.db.ProjectAccounting().GetProjectBandwidthLimit(ctx, projectUUID)
	if err != nil {
		httpJSONError(w, "failed to get bandwidth limit",
			err.Error(), http.StatusInternalServerError)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if err != nil {
		httpJSONError(w, "failed to get project",
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
		Buckets int `json:"maxBuckets"`
	}
	if usagelimit != nil {
		output.Usage.Amount = memory.Size(*usagelimit)
		output.Usage.Bytes = *usagelimit
	}
	if bandwidthlimit != nil {
		output.Bandwidth.Amount = memory.Size(*bandwidthlimit)
		output.Bandwidth.Bytes = *bandwidthlimit
	}
	if project.MaxBuckets != nil {
		output.Buckets = *project.MaxBuckets
	}
	if project.RateLimit != nil {
		output.Rate.RPS = *project.RateLimit
	}

	data, err := json.Marshal(output)
	if err != nil {
		httpJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) putProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	var arguments struct {
		Usage     *memory.Size `schema:"usage"`
		Bandwidth *memory.Size `schema:"bandwidth"`
		Rate      *int         `schema:"rate"`
		Buckets   *int         `schema:"buckets"`
	}

	if err := r.ParseForm(); err != nil {
		httpJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	decoder := schema.NewDecoder()
	err = decoder.Decode(&arguments, r.Form)
	if err != nil {
		httpJSONError(w, "invalid arguments",
			err.Error(), http.StatusBadRequest)
		return
	}

	if arguments.Usage != nil {
		if *arguments.Usage < 0 {
			httpJSONError(w, "negative usage",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectUsageLimit(ctx, projectUUID, *arguments.Usage)
		if err != nil {
			httpJSONError(w, "failed to update usage",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Bandwidth != nil {
		if *arguments.Bandwidth < 0 {
			httpJSONError(w, "negative bandwidth",
				fmt.Sprintf("%v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, projectUUID, *arguments.Bandwidth)
		if err != nil {
			httpJSONError(w, "failed to update bandwidth",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Rate != nil {
		if *arguments.Rate < 0 {
			httpJSONError(w, "negative rate",
				fmt.Sprintf("%v", arguments.Rate), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateRateLimit(ctx, projectUUID, *arguments.Rate)
		if err != nil {
			httpJSONError(w, "failed to update rate",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Buckets != nil {
		if *arguments.Buckets < 0 {
			httpJSONError(w, "negative bucket coun",
				fmt.Sprintf("t: %v", arguments.Buckets), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateBucketLimit(ctx, projectUUID, *arguments.Buckets)
		if err != nil {
			httpJSONError(w, "failed to update bucket limit",
				err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (server *Server) addProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpJSONError(w, "failed to read body",
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
		httpJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.OwnerID.IsZero() {
		httpJSONError(w, "OwnerID is not set",
			"", http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		httpJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Insert(ctx, &console.Project{
		Name:    input.ProjectName,
		OwnerID: input.OwnerID,
	})
	if err != nil {
		httpJSONError(w, "failed to insert project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = server.db.Console().ProjectMembers().Insert(ctx, project.OwnerID, project.ID)
	if err != nil {
		httpJSONError(w, "failed to insert project member",
			err.Error(), http.StatusInternalServerError)
		return
	}

	output.ProjectID = project.ID
	data, err := json.Marshal(output)
	if err != nil {
		httpJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) renameProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if errors.Is(err, sql.ErrNoRows) {
		httpJSONError(w, "project with specified uuid does not exist",
			"", http.StatusBadRequest)
		return
	}
	if err != nil {
		httpJSONError(w, "error getting project",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httpJSONError(w, "ailed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		ProjectName string `json:"projectName"`
		Description string `json:"description"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		httpJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		httpJSONError(w, "ProjectName is not set",
			"", http.StatusBadRequest)
		return
	}

	project.Name = input.ProjectName
	project.Description = input.Description

	err = server.db.Console().Projects().Update(ctx, project)
	if err != nil {
		httpJSONError(w, "error renaming project",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		httpJSONError(w, "project-uuid missing",
			"", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		httpJSONError(w, "invalid project-uuid",
			err.Error(), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		httpJSONError(w, "invalid form",
			err.Error(), http.StatusBadRequest)
		return
	}

	options := storj.BucketListOptions{Limit: 1, Direction: storj.Forward}
	buckets, err := server.db.Buckets().ListBuckets(ctx, projectUUID, options, macaroon.AllowedBuckets{All: true})
	if err != nil {
		httpJSONError(w, "unable to list buckets",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(buckets.Items) > 0 {
		httpJSONError(w, "buckets still exist",
			fmt.Sprintf("%v", bucketNames(buckets.Items)), http.StatusConflict)
		return
	}

	keys, err := server.db.Console().APIKeys().GetPagedByProjectID(ctx, projectUUID, console.APIKeyCursor{Limit: 1, Page: 1})
	if err != nil {
		httpJSONError(w, "unable to list api-keys",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if keys.TotalCount > 0 {
		httpJSONError(w, "api-keys still exist",
			fmt.Sprintf("count %d", keys.TotalCount), http.StatusConflict)
		return
	}

	// if usage exist, return error to client and exit
	if server.checkUsage(ctx, w, projectUUID) {
		return
	}

	err = server.db.Console().Projects().Delete(ctx, projectUUID)
	if err != nil {
		httpJSONError(w, "unable to delete project",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) checkUsage(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID) (hasUsage bool) {
	// do not delete projects that have usage for the current month.
	year, month, _ := server.nowFn().UTC().Date()
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)

	currentUsage, err := server.db.ProjectAccounting().GetProjectTotal(ctx, projectID, firstOfMonth, server.nowFn())
	if err != nil {
		httpJSONError(w, "unable to list project usage",
			err.Error(), http.StatusInternalServerError)
		return true
	}
	if currentUsage.Storage > 0 || currentUsage.Egress > 0 || currentUsage.ObjectCount > 0 {
		httpJSONError(w, "usage for current month exists",
			"", http.StatusConflict)
		return true
	}

	// if usage of last month exist, make sure to look for billing records
	lastMonthUsage, err := server.db.ProjectAccounting().GetProjectTotal(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1))
	if err != nil {
		httpJSONError(w, "error getting project totals",
			"", http.StatusInternalServerError)
		return true
	}

	if lastMonthUsage.Storage > 0 || lastMonthUsage.Egress > 0 || lastMonthUsage.ObjectCount > 0 {
		// time passed into the check function need to be the UTC midnight dates of the first and last day of the month
		err := server.db.StripeCoinPayments().ProjectRecords().Check(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.Add(-time.Hour*24))
		switch err {
		case stripecoinpayments.ErrProjectRecordExists:
			record, err := server.db.StripeCoinPayments().ProjectRecords().Get(ctx, projectID, firstOfMonth.AddDate(0, -1, 0), firstOfMonth.Add(-time.Hour*24))
			if err != nil {
				httpJSONError(w, "unable to get project records",
					err.Error(), http.StatusInternalServerError)
				return true
			}
			// state = 0 means unapplied and not invoiced yet.
			if record.State == 0 {
				httpJSONError(w, "unapplied project invoice record exist",
					"", http.StatusConflict)
				return true
			}
		case nil:
			httpJSONError(w, "usage for last month exist, but is not billed yet",
				"", http.StatusConflict)
			return true
		default:
			httpJSONError(w, "unable to get project records",
				err.Error(), http.StatusInternalServerError)
			return true
		}
	}

	return false
}

func bucketNames(buckets []storj.Bucket) []string {
	var xs []string
	for _, b := range buckets {
		xs = append(xs, b.Name)
	}
	return xs
}
