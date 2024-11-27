// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metasearch

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
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
	ProjectID uuid.UUID               `json:"-"`
	Location  metabase.ObjectLocation `json:"-"`
}

const defaultBatchSize = 100
const maxBatchSize = 1000

// GetRequest contains fields for a get request.
type GetRequest struct {
	BaseRequest
}

// SearchRequest contains fields for a view or search request.
type SearchRequest struct {
	BaseRequest

	KeyPrefix  string                 `json:"keyPrefix,omitempty"`
	Match      map[string]interface{} `json:"match,omitempty"`
	Filter     string                 `json:"filter,omitempty"`
	Projection string                 `json:"projection,omitempty"`

	BatchSize int    `json:"batchSize,omitempty"`
	PageToken string `json:"pageToken,omitempty"`

	startAfter metabase.ObjectStream
}

// SearchResponse contains fields for a view or search response.
type SearchResponse struct {
	Results   []SearchResult `json:"results"`
	PageToken string         `json:"pageToken,omitempty"`
}

// SearchResult contains fields for a single search result.
type SearchResult struct {
	Path     string      `json:"path"`
	Metadata interface{} `json:"metadata"`
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

	// CRUD operations
	router.HandleFunc("/metadata/{bucket}/{key:.*}", s.HandleGet).Methods(http.MethodGet)
	router.HandleFunc("/metadata/{bucket}/{key:.*}", s.HandleUpdate).Methods(http.MethodPut)
	router.HandleFunc("/metadata/{bucket}/{key:.*}", s.HandleDelete).Methods(http.MethodDelete)
	router.HandleFunc("/metasearch/{bucket}", s.HandleQuery).Methods(http.MethodPost)
	s.Handler = router

	return s, nil
}

// Run starts the metasearch server.
func (s *Server) Run() error {
	return http.ListenAndServe(s.Endpoint, s.Handler)
}

func (s *Server) validateRequest(ctx context.Context, r *http.Request, baseRequest *BaseRequest, body interface{}) error {
	// Parse authorization header
	projectID, err := s.Auth.Authenticate(ctx, r)
	if err != nil {
		return err
	}

	// Decode request body
	if body != nil && r.Body != nil {
		if err = json.NewDecoder(r.Body).Decode(body); err != nil {
			return fmt.Errorf("%w: error decoding request body: %w", ErrBadRequest, err)
		}
	}

	// Set location
	vars := mux.Vars(r)
	bucket := vars["bucket"]
	key := vars["key"]
	baseRequest.Location = metabase.ObjectLocation{
		ProjectID:  projectID,
		BucketName: metabase.BucketName(bucket),
		ObjectKey:  metabase.ObjectKey(key),
	}

	return nil
}

func (s *Server) HandleGet(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request BaseRequest

	err := s.validateRequest(ctx, r, &request, nil)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	meta, err := s.Repo.GetMetadata(ctx, request.Location)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	s.jsonResponse(w, http.StatusOK, meta)
}

// HandleQuery handles a metadata view or search request.
func (s *Server) HandleQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request SearchRequest
	var result SearchResponse

	err := s.validateSearchRequest(ctx, r, &request)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	result, err = s.searchMetadata(ctx, &request)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	s.jsonResponse(w, http.StatusOK, result)
}

func (s *Server) validateSearchRequest(ctx context.Context, r *http.Request, request *SearchRequest) error {
	err := s.validateRequest(ctx, r, &request.BaseRequest, request)
	if err != nil {
		return err
	}

	// Validate match query
	if request.Match == nil {
		request.Match = make(map[string]interface{})
	}

	// Validate batch size
	if request.BatchSize <= 0 || request.BatchSize > maxBatchSize {
		request.BatchSize = defaultBatchSize
	}

	// Validate pageToken
	if request.PageToken != "" {
		request.startAfter, err = parsePageToken(request.PageToken)
		if err != nil {
			return err
		}
	}

	// Override key by KeyPrefix parameter
	if request.KeyPrefix != "" {
		request.Location.ObjectKey = metabase.ObjectKey(request.KeyPrefix)
	}

	return nil
}

func (s *Server) searchMetadata(ctx context.Context, request *SearchRequest) (response SearchResponse, err error) {
	searchResult, err := s.Repo.QueryMetadata(ctx, request.Location, request.Match, request.startAfter, request.BatchSize)
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

	// Determine page token
	if len(searchResult.Objects) >= request.BatchSize {
		last := searchResult.Objects[len(searchResult.Objects)-1]
		response.PageToken = getPageToken(last.ObjectStream)
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
	var request BaseRequest
	var metadata map[string]interface{}

	err := s.validateRequest(ctx, r, &request, &metadata)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	err = s.Repo.UpdateMetadata(ctx, request.Location, metadata)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDelete handles a metadata delete request.
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var request BaseRequest

	err := s.validateRequest(ctx, r, &request, nil)
	if err != nil {
		s.errorResponse(w, err)
		return
	}

	err = s.Repo.DeleteMetadata(ctx, request.Location)
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

func getPageToken(obj metabase.ObjectStream) string {
	q := url.Values{}
	q.Set("projectID", obj.ProjectID.String())
	q.Set("bucketName", string(obj.BucketName))
	q.Set("objectKey", string(obj.ObjectKey))
	q.Set("version", strconv.FormatInt(int64(obj.Version), 10))

	return base64.StdEncoding.EncodeToString([]byte(q.Encode()))
}

func parsePageToken(s string) (metabase.ObjectStream, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return metabase.ObjectStream{}, fmt.Errorf("invalid page token: %w", ErrBadRequest)
	}

	q, err := url.ParseQuery(string(b))
	if err != nil {
		return metabase.ObjectStream{}, fmt.Errorf("invalid params in page token: %w", ErrBadRequest)
	}

	projectID, err := uuid.FromString(q.Get("projectID"))
	if err != nil {
		return metabase.ObjectStream{}, fmt.Errorf("invalid projectID in page token: %w", ErrBadRequest)
	}

	bucketName := metabase.BucketName(q.Get("bucketName"))
	if bucketName == "" {
		return metabase.ObjectStream{}, fmt.Errorf("invalid bucketName in page token: %w", ErrBadRequest)
	}

	objectKey := metabase.ObjectKey(q.Get("objectKey"))
	if objectKey == "" {
		return metabase.ObjectStream{}, fmt.Errorf("invalid objectKey in page token: %w", ErrBadRequest)
	}

	version, err := strconv.ParseInt(q.Get("version"), 10, 64)
	if err != nil {
		return metabase.ObjectStream{}, fmt.Errorf("invalid version in page token: %w", ErrBadRequest)
	}

	return metabase.ObjectStream{
		ProjectID:  projectID,
		BucketName: bucketName,
		ObjectKey:  objectKey,
		Version:    metabase.Version(version),
	}, nil
}
