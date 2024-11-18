// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metasearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"storj.io/common/macaroon"
	"storj.io/common/uuid"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/metabase"
)

// Server implements the REST API for metadata search.
type Server struct {
	Logger   *zap.Logger
	Repo     MetaSearchRepo
	Endpoint string
	Router   http.Handler
}

type BaseRequest struct {
	ProjectID uuid.UUID `json:"-"`
	Path      string    `json:"path"`
}

type SearchRequest struct {
	BaseRequest

	Page  int    `json:"page"`
	Query string `json:"query"`
}

type UpdateRequest struct {
	BaseRequest
	Metadata map[string]interface{} `json:"metadata"`
}

type DeleteRequest struct {
	BaseRequest
}

// NewServer creates a new metasearch server process.
func NewServer(log *zap.Logger, db satellite.DB, metabase *metabase.DB, endpoint string) (*Server, error) {
	repo := NewMetabaseSearchRepository(metabase)
	peer := &Server{
		Logger:   log,
		Repo:     repo,
		Endpoint: endpoint,
	}

	return peer, nil
}

func (s *Server) Run() error {
	router := mux.NewRouter()
	router.HandleFunc("/meta_search", s.HandleQuery).Methods(http.MethodPost)
	router.HandleFunc("/meta_search", s.HandleUpdate).Methods(http.MethodPut)
	router.HandleFunc("/meta_search", s.HandleDelete).Methods(http.MethodDelete)
	return http.ListenAndServe(s.Endpoint, router)
}

func (s *Server) validateRequest(ctx context.Context, r *http.Request, baseRequest *BaseRequest, body interface{}) error {
	// Parse authorization header
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		return fmt.Errorf("%w: missing authorization header", ErrAuthorizationFailed)
	}

	// Check for valid authorization
	if !strings.HasPrefix(hdr, "Bearer ") {
		return fmt.Errorf("%w: invalid authorization header", ErrAuthorizationFailed)
	}

	// Parse API token
	rawToken := strings.TrimPrefix(hdr, "Bearer ")
	apiKey, err := macaroon.ParseAPIKey(rawToken)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrAuthorizationFailed, err)
	}
	s.Logger.Info("API key", zap.String("key", fmt.Sprint(apiKey)))

	// Parse project ID from header (TODO: get from API token)
	projectID := r.Header.Get("X-Project-ID")
	if projectID == "" {
		return fmt.Errorf("%w: missing project ID", ErrBadRequest)
	}

	baseRequest.ProjectID, err = uuid.FromString(projectID)
	if err != nil {
		return fmt.Errorf("%w: invalid project ID", ErrBadRequest)
	}

	// Decode request body
	if err = json.NewDecoder(r.Body).Decode(body); err != nil {
		return fmt.Errorf("%w: error decoding request body: %w", ErrBadRequest, err)
	}

	return nil
}

func (s *Server) HandleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request SearchRequest
	var result interface{}

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	if request.Query == "" {
		result, err = s.getMetadata(ctx, &request)
	} else {
		result, err = s.searchMetadata(ctx, &request)
	}

	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	s.JSONResponse(w, http.StatusOK, result)
}

func (s *Server) getMetadata(ctx context.Context, reqBody *SearchRequest) (meta map[string]interface{}, err error) {
	bucket, key, err := parsePath(reqBody.Path)
	if err != nil {
		return nil, err
	}

	loc := metabase.ObjectLocation{
		ProjectID:  reqBody.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	return s.Repo.GetMetadata(ctx, loc)
}

func (s *Server) searchMetadata(ctx context.Context, reqBody *SearchRequest) (keys []string, err error) {
	// Parse path
	bucket, key, err := parsePath(reqBody.Path)
	if err != nil {
		return nil, err
	}

	loc := metabase.ObjectLocation{
		ProjectID:  reqBody.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	// Parse query
	var query map[string]interface{}
	err = json.Unmarshal([]byte(reqBody.Query), &query)
	if err != nil {
		return nil, fmt.Errorf("%w: cannot parse query: %v", ErrBadRequest, err)
	}

	// TODO: implement pagination
	objects, err := s.Repo.QueryMetadata(ctx, loc, query, 1000)
	if err != nil {
		return nil, err
	}

	// Extract keys
	for _, obj := range objects {
		keys = append(keys, string(obj.ObjectKey))
	}

	return keys, nil
}

func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request UpdateRequest

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	bucket, key, err := parsePath(request.Path)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	loc := metabase.ObjectLocation{
		ProjectID:  request.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	err = s.Repo.UpdateMetadata(ctx, loc, request.Metadata)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request DeleteRequest

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	bucket, key, err := parsePath(request.Path)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	loc := metabase.ObjectLocation{
		ProjectID:  request.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	err = s.Repo.DeleteMetadata(ctx, loc)
	if err != nil {
		s.ErrorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) JSONResponse(w http.ResponseWriter, status int, body interface{}) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		s.ErrorResponse(w, fmt.Errorf("%w: %v", ErrInternalError, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonBytes)
}

func (s *Server) ErrorResponse(w http.ResponseWriter, err error) {
	s.Logger.Warn("error during API request", zap.Error(err))

	var e *ErrorResponse
	if !errors.As(err, &e) {
		e = ErrInternalError
	}

	resp, _ := json.Marshal(e)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	w.Write([]byte(resp))
}

func parsePath(path string) (bucket string, key string, err error) {
	if !strings.HasPrefix(path, "sj://") {
		return "", "", fmt.Errorf("invalid path: %w", ErrBadRequest)
	}

	path = strings.TrimPrefix(path, "sj://")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid path: %w", ErrBadRequest)
	}

	return parts[0], parts[1], nil
}
