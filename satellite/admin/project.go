// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"

	"storj.io/common/memory"
	"storj.io/common/uuid"
)

func (server *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		http.Error(w, "user-email missing", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, fmt.Sprintf("user with email %q not found", userEmail), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user %q: %v", userEmail, err), http.StatusInternalServerError)
		return
	}
	user.PasswordHash = nil

	projects, err := server.db.Console().Projects().GetByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user projects %q: %v", userEmail, err), http.StatusInternalServerError)
		return
	}

	type User struct {
		ID       uuid.UUID `json:"id"`
		FullName string    `json:"fullName"`
		Email    string    `json:"email"`
	}
	type Project struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		OwnerID     uuid.UUID `json:"ownerId"`
	}

	var output struct {
		User     User      `json:"user"`
		Projects []Project `json:"projects"`
	}

	output.User = User{
		ID:       user.ID,
		FullName: user.FullName,
		Email:    user.Email,
	}
	for _, p := range projects {
		output.Projects = append(output.Projects, Project{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			OwnerID:     p.OwnerID,
		})
	}

	data, err := json.Marshal(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disapperaed
}

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
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disapperaed
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
