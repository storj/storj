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
}

type SearchRequest struct {
	ProjectID uuid.UUID `json:"-"`

	Page  int    `json:"page"`
	Path  string `json:"path"`
	Query string `json:"query"`
	Meta  string `json:"metadata"`
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

func (a *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var reqBody SearchRequest
	var resp interface{}

	// Parse authorization header
	hdr := r.Header.Get("Authorization")
	if hdr == "" {
		a.ErrorResponse(w, fmt.Errorf("%w: missing authorization header", ErrAuthorizationFailed))
		return
	}

	// Check for valid authorization
	if !strings.HasPrefix(hdr, "Bearer ") {
		a.ErrorResponse(w, fmt.Errorf("%w: invalid authorization header", ErrAuthorizationFailed))
		return
	}

	// Parse API token
	rawToken := strings.TrimPrefix(hdr, "Bearer ")
	apiKey, err := macaroon.ParseAPIKey(rawToken)
	if err != nil {
		a.ErrorResponse(w, fmt.Errorf("%w: %s", ErrAuthorizationFailed, err))
		return
	}
	a.Logger.Info("API key", zap.String("key", fmt.Sprint(apiKey)))

	// Decode request body
	if err = json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		a.ErrorResponse(w, fmt.Errorf("%w: error decoding request body: %w", ErrBadRequest, err))
		return
	}

	// TODO: parse from token
	reqBody.ProjectID, _ = uuid.FromString("97c2848e-017a-460a-9b53-a9ee28c50dc6")

	// Handle request
	ctx := r.Context()
	switch {
	case r.Method == http.MethodPost:
		if reqBody.Query == "" {
			resp, err = a.ViewMetadata(ctx, &reqBody)
		} else {
			resp, err = a.QueryMetadata(&reqBody)
		}
	case r.Method == http.MethodPut:
		err = a.UpdateMetadata(ctx, &reqBody)
	case r.Method == http.MethodDelete:
		err = a.DeleteMetadata(ctx, &reqBody)
	default:
		err = fmt.Errorf("%w: unsupported method %s", ErrBadRequest, r.Method)
	}

	// Write response
	if err != nil {
		a.ErrorResponse(w, err)
		return
	}

	jsonBytes, err := json.Marshal(resp)
	if err != nil {
		a.ErrorResponse(w, fmt.Errorf("error marshalling response: %w", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonBytes)
}

func (a *Server) Run() error {
	mux := http.NewServeMux()

	// Register the routes and handlers
	mux.Handle("/meta_search", a)
	mux.Handle("/meta_search/", a)

	// Run the server
	return http.ListenAndServe(a.Endpoint, mux)
}

func (a *Server) ViewMetadata(ctx context.Context, reqBody *SearchRequest) (meta map[string]interface{}, err error) {
	bucket, key, err := parsePath(reqBody.Path)
	if err != nil {
		return nil, err
	}

	loc := metabase.ObjectLocation{
		ProjectID:  reqBody.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	return a.Repo.GetMetadata(ctx, loc)
}

func (a *Server) QueryMetadata(reqBody *SearchRequest) (meta map[string]interface{}, err error) {
	meta = map[string]interface{}{
		"query": "meta",
	}
	return
}

func (a *Server) UpdateMetadata(ctx context.Context, reqBody *SearchRequest) (err error) {
	bucket, key, err := parsePath(reqBody.Path)
	if err != nil {
		return err
	}

	loc := metabase.ObjectLocation{
		ProjectID:  reqBody.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	meta := make(map[string]interface{})
	err = json.Unmarshal([]byte(reqBody.Meta), &meta)
	if err != nil {
		return fmt.Errorf("%w: cannot parse passed metadata: %v", ErrBadRequest, err)
	}

	return a.Repo.UpdateMetadata(ctx, loc, meta)
}

func (a *Server) DeleteMetadata(ctx context.Context, reqBody *SearchRequest) (err error) {
	bucket, key, err := parsePath(reqBody.Path)
	if err != nil {
		return err
	}

	loc := metabase.ObjectLocation{
		ProjectID:  reqBody.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	return a.Repo.DeleteMetadata(ctx, loc)
}

// ErrorResponse writes an error response to the client.
func (a *Server) ErrorResponse(w http.ResponseWriter, err error) {
	a.Logger.Warn("error during API request", zap.Error(err))

	var e *ErrorResponse
	if !errors.As(err, &e) {
		e = ErrInternalError
	}

	resp, _ := json.Marshal(e)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	w.Write([]byte(resp))
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
