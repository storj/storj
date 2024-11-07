// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metasearch

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
)

// Server implements the REST API for metadata search.
type Server struct {
	Logger      *zap.Logger
	SatelliteDB satellite.DB
	MetabaseDB  *metabase.DB
	Endpoint    string
}

type SearchRequest struct {
	Page  int    `json:"page"`
	Path  string `json:"path"`
	Query string `json:"query"`
	Meta  string `json:"metadata"`
}

// NewServer creates a new metasearch server process.
func NewServer(log *zap.Logger, db satellite.DB, metabase *metabase.DB, endpoint string) (*Server, error) {
	peer := &Server{
		Logger:      log,
		SatelliteDB: db,
		MetabaseDB:  metabase,
		Endpoint:    endpoint,
	}

	return peer, nil
}

func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var reqBody SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		a.Logger.Warn("error decoding request body", zap.Error(err))
		BadRequestHandler(w, r)
		return
	}

	switch {
	case r.Method == http.MethodPost:
		if reqBody.Query != "" {
			a.QueryMetadata(w, r, &reqBody)
			return
		}
		a.ViewMetadata(w, r, &reqBody)
		return
	case r.Method == http.MethodPut:
		a.CreateUpdateMetadata(w, r, &reqBody)
		return
	case r.Method == http.MethodDelete:
		a.DeleteMetadata(w, r, &reqBody)
		return
	default:
		NotFoundHandler(w, r)
		return
	}
}

func (a *Server) Run() error {
	mux := http.NewServeMux()

	// Register the routes and handlers
	mux.Handle("/meta_search", a)
	mux.Handle("/meta_search/", a)

	// Run the server
	return http.ListenAndServe(a.Endpoint, mux)
}

func (a *Server) ViewMetadata(w http.ResponseWriter, r *http.Request, reqBody *SearchRequest) (meta map[string]interface{}, err error) {

	/* TODO add a db repo
	meta, err = h.repo.View(reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}
	*/

	meta = map[string]interface{}{
		"view": "meta",
	}

	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		a.Logger.Error("error marshalling json", zap.Error(err))
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
	return
}

func (a *Server) QueryMetadata(w http.ResponseWriter, r *http.Request, reqBody *SearchRequest) (meta map[string]interface{}, err error) {
	/* TODO add a db repo
	meta, err = h.repo.Query(reqBody.Page, reqBody.Query, reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}
	*/

	meta = map[string]interface{}{
		"query": "meta",
	}

	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		a.Logger.Error("error marshalling json", zap.Error(err))
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
	return
}

func (a *Server) CreateUpdateMetadata(w http.ResponseWriter, r *http.Request, reqBody *SearchRequest) (err error) {
	/* TODO add a db repo
	err = h.repo.CreateUpdate(reqBody.Path, reqBody.Meta)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}
	*/

	w.WriteHeader(http.StatusOK)
	return
}
func (a *Server) DeleteMetadata(w http.ResponseWriter, r *http.Request, reqBody *SearchRequest) (err error) {
	/* TODO add a db repo
	err = h.repo.Delete(reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}
	*/

	w.WriteHeader(http.StatusOK)

	return
}

func BadRequestHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte("400 Bad Request"))
}

func InternalServerErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 Internal Server Error"))
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 Not Found"))
}
