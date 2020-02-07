// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/skyrings/skyring-common/tools/uuid"

	"storj.io/common/memory"
)

func (server *Server) getProjectLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	projectUUIDString, ok := vars["project"]
	if !ok {
		http.Error(w, "project-uuid missing", http.StatusBadRequest)
		return
	}

	projectUUID, err := uuid.Parse(projectUUIDString)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid project-uuid: %v", err), http.StatusBadRequest)
		return
	}

	limit, err := server.db.ProjectAccounting().GetProjectStorageLimit(ctx, *projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get usage limit: %v", err), http.StatusInternalServerError)
		return
	}

	project, err := server.db.Console().Projects().Get(ctx, *projectUUID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get project: %v", err), http.StatusInternalServerError)
		return
	}

	var output struct {
		Usage struct {
			Amount memory.Size `json:"amount"`
			Bytes  int64       `json:"bytes"`
		} `json:"usage"`
		Rate struct {
			RPS int `json:"rps"`
		} `json:"rate"`
	}
	output.Usage.Amount = limit
	output.Usage.Bytes = limit.Int64()
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

	projectUUID, err := uuid.Parse(projectUUIDString)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid project-uuid: %v", err), http.StatusBadRequest)
		return
	}

	var arguments struct {
		Usage *memory.Size `schema:"usage"`
		Rate  *int         `schema:"rate"`
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

		err = server.db.ProjectAccounting().UpdateProjectUsageLimit(ctx, *projectUUID, *arguments.Usage)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to update usage: %v", err), http.StatusInternalServerError)
			return
		}
	}

	if arguments.Rate != nil {
		if *arguments.Rate < 0 {
			http.Error(w, fmt.Sprintf("negative rate: %v", arguments.Rate), http.StatusBadRequest)
			return
		}

		err = server.db.Console().Projects().UpdateRateLimit(ctx, *projectUUID, *arguments.Rate)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to update rate: %v", err), http.StatusInternalServerError)
			return
		}
	}
}
