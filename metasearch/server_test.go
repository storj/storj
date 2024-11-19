package metasearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/assert"
	"go.uber.org/zap"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/metabase"
)

// Mock repository

type mockRepo struct {
	metadata map[string]map[string]interface{}
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		metadata: make(map[string]map[string]interface{}),
	}
}

func (r *mockRepo) GetMetadata(ctx context.Context, loc metabase.ObjectLocation) (map[string]interface{}, error) {
	path := fmt.Sprintf("sj://%s/%s", loc.BucketName, loc.ObjectKey)
	m, ok := r.metadata[path]
	if !ok {
		return nil, ErrNotFound
	}
	return m, nil
}

func (r *mockRepo) UpdateMetadata(ctx context.Context, loc metabase.ObjectLocation, meta map[string]interface{}) error {
	path := fmt.Sprintf("sj://%s/%s", loc.BucketName, loc.ObjectKey)
	r.metadata[path] = meta
	return nil
}

func (r *mockRepo) DeleteMetadata(ctx context.Context, loc metabase.ObjectLocation) error {
	path := fmt.Sprintf("sj://%s/%s", loc.BucketName, loc.ObjectKey)
	delete(r.metadata, path)
	return nil
}

func (r *mockRepo) QueryMetadata(ctx context.Context, loc metabase.ObjectLocation, match map[string]interface{}, batchSize int) (metabase.FindObjectsByClearMetadataResult, error) {
	results := metabase.FindObjectsByClearMetadataResult{}
	path := fmt.Sprintf("sj://%s/%s", loc.BucketName, loc.ObjectKey)

	// return all objects whose path starts with the `loc`
	for k, v := range r.metadata {
		if !strings.HasPrefix(k, path) {
			continue
		}

		buf, _ := json.Marshal(v)
		bucket, key, _ := parsePath(k)
		results.Objects = append(results.Objects, metabase.FindObjectsByClearMetadataResultObject{
			ObjectStream: metabase.ObjectStream{
				ProjectID:  loc.ProjectID,
				BucketName: metabase.BucketName(bucket),
				ObjectKey:  metabase.ObjectKey(key),
				Version:    metabase.Version(0),
				StreamID:   uuid.UUID{},
			},
			ClearMetadata: string(buf),
		})

	}
	return results, nil
}

// Mock authentication

type mockAuth struct{}

func (a *mockAuth) Authenticate(r *http.Request) error {
	return nil
}

// Utility functions

const testProjectID = "00000000-0000-0000-0000-000000000000"

func testServer() *Server {
	repo := newMockRepo()
	auth := &mockAuth{}
	logger, _ := zap.NewDevelopment()
	server, _ := NewServer(logger, repo, auth, "")
	return server
}

func testRequest(method, body string) *http.Request {
	var r *http.Request
	url := "/service/meta_search"
	if body != "" {
		r, _ = http.NewRequest(method, url, strings.NewReader(body))
	} else {
		r, _ = http.NewRequest(method, url, nil)
	}
	r.Header.Set("Authorization", "Bearer testtoken")
	r.Header.Set("X-Project-ID", testProjectID)
	return r
}

func assertResponse(t *testing.T, rr *httptest.ResponseRecorder, code int, body string) {
	assert.Equal(t, rr.Code, code)
	if body == "" {
		return
	}

	actualBody, _ := io.ReadAll(rr.Body)
	require.JSONEq(t, body, string(actualBody))
}

func responseJSON(t *testing.T, rr *httptest.ResponseRecorder) map[string]interface{} {
	var m map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&m)
	require.Nil(t, err)
	return m
}

// Unit tests

func TestMetaSearchCRUD(t *testing.T) {
	server := testServer()

	// Insert metadata
	rr := httptest.NewRecorder()
	r := testRequest(http.MethodPut, `{
		"path": "sj://testbucket/foo.txt",
		"metadata": {
			"foo": "456",
			"n": 2,
			"tags": [
				"tag1",
				"tag3"
			]
		}
	}`)
	server.HandleUpdate(rr, r)
	assert.Equal(t, rr.Code, http.StatusNoContent)

	// Get metadata
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodPost, `{
		"path": "sj://testbucket/foo.txt"
	}`)
	server.HandleQuery(rr, r)
	assertResponse(t, rr, http.StatusOK, `{
		"results": [{
			"path": "sj://testbucket/foo.txt",
			"metadata": {
				"foo": "456",
				"n": 2,
				"tags": [
					"tag1",
					"tag3"
				]
			}
		}]
	}`)

	// Delete metadata
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodDelete, `{
		"path": "sj://testbucket/foo.txt"
	}`)
	server.HandleDelete(rr, r)
	assert.Equal(t, rr.Code, http.StatusNoContent)

	// Get metadata again
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodPost, `{
		"path": "sj://testbucket/foo.txt"
	}`)
	server.HandleQuery(rr, r)
	assertResponse(t, rr, http.StatusNotFound, `{
		"error": "not found"
	}`)
}

func TestMetaSearchQuery(t *testing.T) {
	server := testServer()

	// Insert metadata
	rr := httptest.NewRecorder()
	r := testRequest(http.MethodPut, `{
		"path": "sj://testbucket/foo.txt",
		"metadata": {
			"foo": "456",
			"n": 1
		}
	}`)
	server.HandleUpdate(rr, r)
	assert.Equal(t, rr.Code, http.StatusNoContent)

	r = testRequest(http.MethodPut, `{
		"path": "sj://testbucket/bar.txt",
		"metadata": {
			"foo": "456",
			"n": 2
		}
	}`)
	server.HandleUpdate(rr, r)
	assert.Equal(t, rr.Code, http.StatusNoContent)

	// Query with match only
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodPost, `{
		"path": "sj://testbucket/",
		"match": {
			"foo": "456"
		}
	}`)
	server.HandleQuery(rr, r)
	assert.Equal(t, rr.Code, http.StatusOK)
	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.Nil(t, err)
	require.Len(t, resp["results"], 2)

	// Query with match and filter
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodPost, `{
		"path": "sj://testbucket/",
		"match": {
			"foo": "456"
		},
		"filter": "n > `+"`1`"+`"
	}`)
	server.HandleQuery(rr, r)
	assertResponse(t, rr, http.StatusOK, `{
		"results": [{
			"path": "sj://testbucket/bar.txt",
			"metadata": {
				"foo": "456",
				"n": 2
			}
		}]
	}`)

	// Query with match, filter and projection
	rr = httptest.NewRecorder()
	r = testRequest(http.MethodPost, `{
		"path": "sj://testbucket/",
		"match": {
			"foo": "456"
		},
		"filter": "n > `+"`1`"+`",
		"projection": "n"
	}`)
	server.HandleQuery(rr, r)
	assertResponse(t, rr, http.StatusOK, `{
		"results": [{
			"path": "sj://testbucket/bar.txt",
			"metadata": 2
		}]
	}`)
}
