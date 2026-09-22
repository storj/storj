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

	"storj.io/storj/satellite/mailservice/htmltext"
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
	for _, source := range s.sources {
		generated, err := os.ReadFile(filepath.Join(p.dir, strings.TrimSuffix(source.name, ".html")+".txt"))
		require.NoError(t, err)
		require.Equal(t, htmltext.Convert(strings.NewReader(string(source.content))), string(generated),
			"run go generate ./satellite/mailservice/ after changing %s", source.name)
	}
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
			html, err := s.render(name, false)
			require.NoError(t, err)
			plainText, err := s.render(name, true)
			require.NoError(t, err)
			// Extracting text after rendering and rendering the generated text
			// must agree, including links and hidden preheaders in partials.
			require.Equal(t,
				strings.Fields(htmltext.Convert(strings.NewReader(string(html)))),
				strings.Fields(string(plainText)),
				"HTML/text mismatch for %s, branch %v", name, branch)
			for _, plain := range []bool{false, true} {
				content, err := s.render(name, plain)
				require.NoError(t, err, "template %s, plain %v, branch %v", name, plain, branch)
				require.NotEmpty(t, content)
				require.NotContains(t, string(content), "<no value>")
				if plain {
					// Layout definitions render nothing; their newlines must not pad the message.
					require.NotContains(t, string(content), "\n\n\n", "blank lines in %s", name)
				}
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
	require.Equal(t, "After!\n\nHello Test\n", text.Body.String())

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

func TestTemplateEscaping(t *testing.T) {
	s, err := (preview{dir: "../../../web/satellite/static/emails"}).read()
	require.NoError(t, err)
	var data map[string]any
	require.NoError(t, json.Unmarshal(sample, &data))
	data["BrandName"] = "<b>Not markup</b>"
	data["PrimaryColor"] = "red;position:fixed"
	data["Data"].(map[string]any)["ResetLink"] = "javascript:alert(1)"
	s.data, err = json.Marshal(data)
	require.NoError(t, err)
	rendered, err := s.render("Forgot", false)
	require.NoError(t, err)
	require.Contains(t, string(rendered), "&lt;b&gt;Not markup&lt;/b&gt;")
	require.Contains(t, string(rendered), `href="#ZgotmplZ"`)
	require.NotContains(t, string(rendered), "<b>Not markup</b>")
	_, body, ok := strings.Cut(string(rendered), "<body")
	require.True(t, ok)
	require.NotContains(t, body, "red;position:fixed")
}

func TestSampleNumbers(t *testing.T) {
	s := snapshot{
		sources: []source{{name: "Numbers.html", content: []byte(`<p>{{if eq .Count 2}}{{printf "%.2f" .Price}}{{end}} {{range .Items}}{{if eq .Count 3}}{{printf "%.1f" .Price}}{{end}}{{end}} {{range .Nested}}{{range .}}{{if eq . 4}}nested{{end}}{{end}}{{end}}</p>`)}},
		data:    []byte(`{"Count":2,"Price":12.75,"Items":[{"Count":3,"Price":1.5}],"Nested":[[4]]}`),
	}
	for _, plain := range []bool{false, true} {
		rendered, err := s.render("Numbers", plain)
		require.NoError(t, err)
		require.Contains(t, string(rendered), "12.75 1.5 nested")
	}
}
