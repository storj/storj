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
	"github.com/jmespath/go-jmespath"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// Server implements the REST API for metadata search.
type Server struct {
	Logger   *zap.Logger
	Repo     MetaSearchRepo
	Auth     Auth
	Endpoint string
	Handler  http.Handler
}

// BaseRequest contains common fields for all requests.
type BaseRequest struct {
	ProjectID uuid.UUID `json:"-"`
	Path      string    `json:"path"`

	location metabase.ObjectLocation
}

// SearchRequest contains fields for a view or search request.
type SearchRequest struct {
	BaseRequest

	Match      map[string]interface{} `json:"match"`
	Filter     string                 `json:"filter"`
	Projection string                 `json:"projection"`
}

// SearchResponse contains fields for a view or search response.
type SearchResponse struct {
	Results []SearchResult `json:"results"`
}

// SearchResult contains fields for a single search result.
type SearchResult struct {
	Path     string      `json:"path"`
	Metadata interface{} `json:"metadata"`
}

// UpdateRequest contains fields for an update request.
type UpdateRequest struct {
	BaseRequest
	Metadata map[string]interface{} `json:"metadata"`
}

// DeleteRequest contains fields for a delete request.
type DeleteRequest struct {
	BaseRequest
}

// NewServer creates a new metasearch server process.
func NewServer(log *zap.Logger, repo MetaSearchRepo, auth Auth, endpoint string) (*Server, error) {
	s := &Server{
		Logger:   log,
		Repo:     repo,
		Auth:     auth,
		Endpoint: endpoint,
	}

	router := mux.NewRouter()
	router.HandleFunc("/meta_search", s.HandleQuery).Methods(http.MethodPost)
	router.HandleFunc("/meta_search", s.HandleUpdate).Methods(http.MethodPut)
	router.HandleFunc("/meta_search", s.HandleDelete).Methods(http.MethodDelete)
	s.Handler = router

	return s, nil
}

// Run starts the metasearch server.
func (s *Server) Run() error {
	return http.ListenAndServe(s.Endpoint, s.Handler)
}

func (s *Server) validateRequest(ctx context.Context, r *http.Request, baseRequest *BaseRequest, body interface{}) error {
	// Parse authorization header
	err := s.Auth.Authenticate(r)
	if err != nil {
		return err
	}

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

	// Decode path
	bucket, key, err := parsePath(baseRequest.Path)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrBadRequest, err)
	}
	baseRequest.location = metabase.ObjectLocation{
		ProjectID:  baseRequest.ProjectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	return nil
}

// HandleQuery handles a metadata view or search request.
func (s *Server) HandleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request SearchRequest
	var result SearchResponse

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	if request.Match == nil {
		result, err = s.getMetadata(ctx, &request)
	} else {
		result, err = s.searchMetadata(ctx, &request)
	}

	if err != nil {
		s.errorResponse(w, err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func (s *Server) getMetadata(ctx context.Context, request *SearchRequest) (response SearchResponse, err error) {
	meta, err := s.Repo.GetMetadata(ctx, request.location)
	if err == nil {
		response.Results = []SearchResult{
			{
				Path:     request.Path,
				Metadata: meta,
			},
		}
	}
	return
}

func (s *Server) searchMetadata(ctx context.Context, request *SearchRequest) (response SearchResponse, err error) {
	searchResult, err := s.Repo.QueryMetadata(ctx, request.location, request.Match, 1000)
	if err != nil {
		return
	}

	// Extract keys
	var metadata map[string]interface{}
	var projectedMetadata interface{}

	var shouldInclude bool
	response.Results = make([]SearchResult, 0)
	for _, obj := range searchResult.Objects {
		// Parse metadata
		metadata, err = parseJSON(obj.ClearMetadata)
		if err != nil {
			return
		}

		// Apply filter
		shouldInclude, err = s.filterMetadata(ctx, request, metadata)
		if err != nil {
			return
		}
		if !shouldInclude {
			continue
		}

		// Apply projection
		if request.Projection != "" {
			projectedMetadata, err = jmespath.Search(request.Projection, metadata)
		} else {
			projectedMetadata = metadata
		}
		if err != nil {
			return
		}

		response.Results = append(response.Results, SearchResult{
			Path:     fmt.Sprintf("sj://%s/%s", obj.BucketName, obj.ObjectKey),
			Metadata: projectedMetadata,
		})
	}
	return
}

func (s *Server) filterMetadata(ctx context.Context, request *SearchRequest, metadata map[string]interface{}) (bool, error) {
	if request.Filter == "" {
		return true, nil
	}

	// Evaluate JMESPath filter
	result, err := jmespath.Search(request.Filter, metadata)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrBadRequest, err)
	}

	// Check if result is a boolean
	if b, ok := result.(bool); ok {
		return b, nil
	}

	// Check if result is nil
	if result == nil {
		return false, nil
	}

	// Include metadata if result is not nil or false
	return true, nil
}

// HandleUpdate handles a metadata update request.
func (s *Server) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request UpdateRequest

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	err = s.Repo.UpdateMetadata(ctx, request.location, request.Metadata)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDelete handles a metadata delete request.
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request DeleteRequest

	err := s.validateRequest(ctx, r, &request.BaseRequest, &request)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	err = s.Repo.DeleteMetadata(ctx, request.location)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) jsonResponse(w http.ResponseWriter, status int, body interface{}) {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		s.errorResponse(w, fmt.Errorf("%w: %v", ErrInternalError, err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(jsonBytes)
}

func (s *Server) errorResponse(w http.ResponseWriter, err error) {
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
