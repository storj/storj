// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

// Email preview serves template-backed emails locally with sample data and live reload.
package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	texttemplate "text/template"
	"time"

	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/mailservice/htmltext"
)

//go:embed index.html
var index []byte

//go:embed sample.json
var sample []byte

type preview struct {
	dir      string
	dataPath string
}

type source struct {
	name    string
	content []byte
}

type snapshot struct {
	sources  []source
	data     []byte
	revision string
	names    []string
}

func main() {
	addr := flag.String("addr", "127.0.0.1:8025", "HTTP listen address")
	dir := flag.String("dir", "web/satellite/static/emails", "email template directory (run from repository root)")
	data := flag.String("data", "", "optional JSON sample data file, reloaded on edits")
	flag.Parse()
	p := preview{dir: *dir, dataPath: *data}
	if _, err := p.read(); err != nil {
		log.Fatal(err)
	}
	log.Printf("Email preview: http://%s (watching %s)", *addr, *dir)
	server := &http.Server{Addr: *addr, Handler: p.handler(), ReadHeaderTimeout: 5 * time.Second}
	log.Fatal(server.ListenAndServe())
}

// read takes a fresh snapshot, including shared partials and sample data. Content
// hashes detect rapid saves even when the file size and modification time match.
func (p preview) read() (snapshot, error) {
	s := snapshot{data: sample, names: []string{}}
	files, err := filepath.Glob(filepath.Join(p.dir, "*.html"))
	if err != nil {
		return s, err
	}
	if len(files) == 0 {
		return s, fmt.Errorf("no HTML templates found in %s", p.dir)
	}
	hash := sha256.New()
	for _, path := range files {
		content, err := os.ReadFile(path)
		if err != nil {
			return s, err
		}
		name := filepath.Base(path)
		s.sources = append(s.sources, source{name: name, content: content})
		_, _ = fmt.Fprintf(hash, "%s\x00%s\x00", name, content)
		if !strings.HasPrefix(name, "_") {
			s.names = append(s.names, strings.TrimSuffix(name, ".html"))
		}
	}
	if p.dataPath != "" {
		s.data, err = os.ReadFile(p.dataPath)
		if err != nil {
			return s, err
		}
	}
	_, _ = hash.Write(s.data)
	s.revision = fmt.Sprintf("%x", hash.Sum(nil))
	return s, nil
}

func (p preview) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		s, err := p.read()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			Revision string   `json:"revision"`
			Names    []string `json:"names"`
		}{s.revision, s.names})
	})
	mux.HandleFunc("GET /render/{name}", func(w http.ResponseWriter, r *http.Request) {
		s, err := p.read()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		name := r.PathValue("name")
		found := false
		for _, n := range s.names {
			if n == name {
				found = true
				break
			}
		}
		if !found {
			http.NotFound(w, r)
			return
		}
		plain := r.URL.Query().Get("format") == "text"
		content, err := s.render(name, plain)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		contentType := "text/html; charset=utf-8"
		if plain {
			contentType = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write(content)
	})
	// Match the asset URLs used by production templates without requiring a satellite.
	mux.Handle("GET /static/static/", http.StripPrefix("/static/static/", http.FileServer(http.Dir(filepath.Dir(p.dir)))))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		mux.ServeHTTP(w, r)
	})
}

func (s snapshot) render(name string, plain bool) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(s.data))
	decoder.UseNumber()
	var data map[string]any
	if err := decoder.Decode(&data); err != nil {
		return nil, fmt.Errorf("sample data: %w", err)
	}
	if data == nil {
		return nil, fmt.Errorf("sample data must be a JSON object")
	}
	if err := convertNumbers(data); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if plain {
		t := texttemplate.New("emails").Funcs(mailservice.TemplateFuncs).Option("missingkey=error")
		for _, src := range s.sources {
			if _, err := t.New(src.name).Parse(htmltext.Convert(bytes.NewReader(src.content))); err != nil {
				return nil, err
			}
		}
		if err := t.ExecuteTemplate(&buf, name+".html", data); err != nil {
			return nil, err
		}
		return []byte(htmltext.CollapseBlankLines(buf.String())), nil
	} else {
		t := template.New("emails").Funcs(mailservice.TemplateFuncs).Option("missingkey=error")
		for _, src := range s.sources {
			if _, err := t.New(src.name).Parse(string(src.content)); err != nil {
				return nil, err
			}
		}
		if err := t.ExecuteTemplate(&buf, name+".html", data); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// Template comparisons such as eq .Data.EmailNumber 1 need integer values,
// rather than the floating-point numbers used by json.Unmarshal by default.
func convertNumbers(data map[string]any) error {
	for key, value := range data {
		switch value := value.(type) {
		case json.Number:
			n, err := value.Int64()
			if err != nil {
				return fmt.Errorf("sample field %s must be an integer: %w", key, err)
			}
			data[key] = n
		case map[string]any:
			if err := convertNumbers(value); err != nil {
				return err
			}
		}
	}
	return nil
}
