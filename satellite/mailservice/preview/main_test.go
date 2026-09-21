// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func request(handler http.Handler, path string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil))
	return response
}

func TestRepositoryTemplates(t *testing.T) {
	p := preview{dir: "../../../web/satellite/static/emails"}
	s, err := p.read()
	require.NoError(t, err)
	require.NotEmpty(t, s.names)
	branches := []map[string]any{
		{},
		{"EmailNumber": 2, "Days": 0, "IsFree": false},
		{"EmailNumber": 3, "Days": -1, "IsFree": false, "IsNFR": true},
	}
	for _, branch := range branches {
		var data map[string]any
		require.NoError(t, json.Unmarshal(sample, &data))
		for key, value := range branch {
			data["Data"].(map[string]any)[key] = value
		}
		s.data, err = json.Marshal(data)
		require.NoError(t, err)
		for _, name := range s.names {
			for _, plain := range []bool{false, true} {
				content, err := s.render(name, plain)
				require.NoError(t, err, "template %s, plain %v, branch %v", name, plain, branch)
				require.NotEmpty(t, content)
				require.NotContains(t, string(content), "<no value>")
			}
		}
	}
	handler := p.handler()
	require.Equal(t, http.StatusOK, request(handler, "/").Code)
	require.Equal(t, http.StatusOK, request(handler, "/static/static/images/emails/storj-logo.png").Code)
	require.Equal(t, http.StatusNotFound, request(handler, "/render/_footer").Code)
	require.Equal(t, http.StatusNotFound, request(handler, "/render/unknown").Code)
}

func TestLiveEdits(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0600))
	}
	write("Welcome.html", `{{template "header" .}}<p>Hello {{.BrandName}}</p>`)
	write("_header.html", `{{define "header"}}<h1>Before</h1>{{end}}`)
	write("data.json", `{"BrandName":"Test"}`)
	p := preview{dir: dir, dataPath: filepath.Join(dir, "data.json")}
	handler := p.handler()
	revision := func() string {
		t.Helper()
		response := request(handler, "/api/state")
		require.Equal(t, http.StatusOK, response.Code)
		var state struct {
			Revision string
			Names    []string
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &state))
		require.NotContains(t, state.Names, "_header")
		return state.Revision
	}
	original := revision()
	response := request(handler, "/render/Welcome")
	require.Equal(t, http.StatusOK, response.Code)
	require.Contains(t, response.Body.String(), "Before")
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	// Editors can preserve timestamps and byte lengths; detect actual contents.
	info, err := os.Stat(filepath.Join(dir, "_header.html"))
	require.NoError(t, err)
	write("_header.html", `{{define "header"}}<h1>After!</h1>{{end}}`)
	require.NoError(t, os.Chtimes(filepath.Join(dir, "_header.html"), info.ModTime(), info.ModTime()))
	require.NotEqual(t, original, revision())
	require.Contains(t, request(handler, "/render/Welcome").Body.String(), "After!")
	text := request(handler, "/render/Welcome?format=text")
	require.Equal(t, "text/plain; charset=utf-8", text.Header().Get("Content-Type"))
	require.Equal(t, "\nAfter!\n\nHello Test\n", text.Body.String())

	beforeData := revision()
	write("data.json", `{"BrandName":"Changed"}`)
	require.NotEqual(t, beforeData, revision())
	require.Contains(t, request(handler, "/render/Welcome").Body.String(), "Hello Changed")

	write("_header.html", `{{define "header"}}`)
	require.Equal(t, http.StatusInternalServerError, request(handler, "/render/Welcome").Code)
	write("_header.html", `{{define "header"}}Fixed{{end}}`)
	require.Equal(t, http.StatusOK, request(handler, "/render/Welcome").Code)
	write("data.json", `{`)
	require.Equal(t, http.StatusInternalServerError, request(handler, "/render/Welcome").Code)
	write("data.json", `{}`)
	require.Equal(t, http.StatusInternalServerError, request(handler, "/render/Welcome").Code)
	write("data.json", `{"BrandName":"Recovered"}`)
	require.Contains(t, request(handler, "/render/Welcome").Body.String(), "Recovered")

	beforeAdd := revision()
	write("Added.html", "New email")
	require.NotEqual(t, beforeAdd, revision())
	require.Contains(t, request(handler, "/api/state").Body.String(), `"Added"`)
	require.NoError(t, os.Remove(filepath.Join(dir, "Added.html")))
	require.Equal(t, beforeAdd, revision())
	require.Equal(t, http.StatusNotFound, request(handler, "/render/Added").Code)
	// A body edit also updates both formats without regenerating .txt files.
	write("Welcome.html", strings.ReplaceAll(`{{template "header" .}}<p>Hello {{.BrandName}}</p>`, "Hello", "Welcome"))
	require.Contains(t, request(handler, "/render/Welcome?format=text").Body.String(), "Welcome Recovered")
}
