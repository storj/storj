// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"

	"storj.io/common/macaroon"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

func (server *Server) getProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		http.Error(w, "project-uuid missing", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid project-uuid: %v", err), http.StatusBadRequest)
		return
	}

	usagelimit, err := server.db.ProjectAccounting().GetProjectStorageLimit(ctx, projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get usage limit: %v", err), http.StatusInternalServerError)
		return
	}

	bandwidthlimit, err := server.db.ProjectAccounting().GetProjectBandwidthLimit(ctx, projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get bandwidth limit: %v", err), http.StatusInternalServerError)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get project: %v", err), http.StatusInternalServerError)
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
	}
	output.Usage.Amount = usagelimit
	output.Usage.Bytes = usagelimit.Int64()
	output.Bandwidth.Amount = bandwidthlimit
	output.Bandwidth.Bytes = bandwidthlimit.Int64()
	if project.RateLimit != nil {
		output.Rate.RPS = *project.RateLimit
	}

	data, err := json.Marshal(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
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
		http.Error(w, "project-uuid missing", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid project-uuid: %v", err), http.StatusBadRequest)
		return
	}

	var arguments struct {
		Usage     *memory.Size `schema:"usage"`
		Bandwidth *memory.Size `schema:"bandwidth"`
		Rate      *int         `schema:"rate"`
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("invalid form: %v", err), http.StatusBadRequest)
		return
	}

	decoder := schema.NewDecoder()
	err = decoder.Decode(&arguments, r.Form)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid arguments: %v", err), http.StatusBadRequest)
		return
	}

	if arguments.Usage != nil {
		if *arguments.Usage < 0 {
			http.Error(w, fmt.Sprintf("negative usage: %v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectUsageLimit(ctx, projectUUID, *arguments.Usage)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to update usage: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Bandwidth != nil {
		if *arguments.Bandwidth < 0 {
			http.Error(w, fmt.Sprintf("negative bandwidth: %v", arguments.Usage), http.StatusBadRequest)
			return
		}

		err = server.db.ProjectAccounting().UpdateProjectBandwidthLimit(ctx, projectUUID, *arguments.Bandwidth)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to update bandwidth: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Rate != nil {
		if *arguments.Rate < 0 {
			http.Error(w, fmt.Sprintf("negative rate: %v", arguments.Rate), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateRateLimit(ctx, projectUUID, *arguments.Rate)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to update rate: %v", err), http.StatusInternalServerError)
			return
		}
	}
}

func (server *Server) addProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
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
		http.Error(w, fmt.Sprintf("failed to unmarshal request: %v", err), http.StatusBadRequest)
		return
	}

	if input.OwnerID.IsZero() {
		http.Error(w, "OwnerID is not set", http.StatusBadRequest)
		return
	}

	if input.ProjectName == "" {
		http.Error(w, "ProjectName is not set", http.StatusBadRequest)
		return
	}

	project, err := server.db.Console().Projects().Insert(ctx, &console.Project{
		Name:    input.ProjectName,
		OwnerID: input.OwnerID,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to insert project: %v", err), http.StatusInternalServerError)
		return
	}

	_, err = server.db.Console().ProjectMembers().Insert(ctx, project.OwnerID, project.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to insert project member: %v", err), http.StatusInternalServerError)
		return
	}

	output.ProjectID = project.ID
	data, err := json.Marshal(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) deleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		http.Error(w, "project-uuid missing", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.FromString(projectUUIDString)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid project-uuid: %v", err), http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, fmt.Sprintf("invalid form: %v", err), http.StatusBadRequest)
		return
	}

	buckets, err := server.db.Buckets().ListBuckets(ctx, projectUUID, storj.BucketListOptions{Limit: 1, Direction: storj.Forward}, macaroon.AllowedBuckets{All: true})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to list buckets: %v", err), http.StatusInternalServerError)
		return
	}
	if len(buckets.Items) > 0 {
		http.Error(w, fmt.Sprintf("buckets still exist: %v", bucketNames(buckets.Items)), http.StatusConflict)
		return
	}

	keys, err := server.db.Console().APIKeys().GetPagedByProjectID(ctx, projectUUID, console.APIKeyCursor{Limit: 1, Page: 1})
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to list api-keys: %v", err), http.StatusInternalServerError)
		return
	}
	if keys.TotalCount > 0 {
		http.Error(w, fmt.Sprintf("api-keys still exist: count %v", keys.TotalCount), http.StatusConflict)
		return
	}

	err = server.db.Console().Projects().Delete(ctx, projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("unable to delete project: %v", err), http.StatusInternalServerError)
		return
	}
}

func bucketNames(buckets []storj.Bucket) []string {
	var xs []string
	for _, b := range buckets {
		xs = append(xs, b.Name)
	}
	return xs
}
