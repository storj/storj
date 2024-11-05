// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metadata

import (
	"encoding/json"
	"net/http"
	"regexp"

	"go.uber.org/zap"

	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
)

var (
	MetaSearchRe = regexp.MustCompile(`^/meta_search/*$`)
)

// API is the metadata API process.
type API struct {
	Log         *zap.Logger
	StaelliteDB satellite.DB
	MetabaseDB  *metabase.DB
	Endpoint    string
}

type RequestBody struct {
	Page  int    `json:"page"`
	Path  string `json:"path"`
	Query string `json:"query"`
	Meta  string `json:"metadata"`
}

func (r *RequestBody) Valid() bool {
	return r.Path != ""
}

// metadata search repo represent a collection of operations on metadata
type MetaSearchRepo interface {
	View(path string) (meta string, err error)
	Query(page int, query, path string) (meta string, err error)
	CreateUpdate(path, metadata string) (err error)
	Delete(path string) (err error)
}

// metadata search handler implements http.Handler and dispatch request to the repo
type MetaSearchHandler struct {
	repo MetaSearchRepo
}

func InternalServerErrorHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte("500 Internal Server Error"))
}

func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("404 Not Found"))
}

func NewMetaSearchHandler(r MetaSearchRepo) *MetaSearchHandler {
	return &MetaSearchHandler{
		repo: r,
	}
}

func (h *MetaSearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var reqBody RequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		InternalServerErrorHandler(w, r)
		return
	}

	if !reqBody.Valid() {
		InternalServerErrorHandler(w, r)
		return
	}

	switch {
	case r.Method == http.MethodPost && MetaSearchRe.MatchString(r.URL.Path):
		if reqBody.Query != "" {
			h.QueryMetadata(w, r, &reqBody)
			return
		}
		h.ViewMetadata(w, r, &reqBody)
		return
	case r.Method == http.MethodPut && MetaSearchRe.MatchString(r.URL.Path):
		h.CreateUpdateMetadata(w, r, &reqBody)
		return
	case r.Method == http.MethodDelete && MetaSearchRe.MatchString(r.URL.Path):
		h.DeleteMetadata(w, r, &reqBody)
		return
	default:
		NotFoundHandler(w, r)
		return
	}
}

// NewAPI creates a new metadata API process.
func NewAPI(log *zap.Logger, db satellite.DB, metabase *metabase.DB, endpoint string) (*API, error) {
	peer := &API{
		Log:         log,
		StaelliteDB: db,
		MetabaseDB:  metabase,
		Endpoint:    endpoint,
	}

	return peer, nil
}

func (a *API) Run() {
	mux := http.NewServeMux()

	// Register the routes and handlers
	mux.Handle("/meta_search", &MetaSearchHandler{})
	mux.Handle("/meta_search/", &MetaSearchHandler{})

	// Run the server
	http.ListenAndServe(a.Endpoint, mux)
}

func (h *MetaSearchHandler) ViewMetadata(w http.ResponseWriter, r *http.Request, reqBody *RequestBody) (meta string, err error) {
	meta, err = h.repo.View(reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (h *MetaSearchHandler) QueryMetadata(w http.ResponseWriter, r *http.Request, reqBody *RequestBody) (meta string, err error) {
	meta, err = h.repo.Query(reqBody.Page, reqBody.Query, reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}

func (h *MetaSearchHandler) CreateUpdateMetadata(w http.ResponseWriter, r *http.Request, reqBody *RequestBody) (err error) {
	err = h.repo.CreateUpdate(reqBody.Path, reqBody.Meta)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)
	return
}
func (h *MetaSearchHandler) DeleteMetadata(w http.ResponseWriter, r *http.Request, reqBody *RequestBody) (err error) {
	err = h.repo.Delete(reqBody.Path)

	if err != nil {
		InternalServerErrorHandler(w, r)
		return
	}

	w.WriteHeader(http.StatusOK)

	return
}
