// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package debug

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"text/template"
	"unicode"

	"go.uber.org/zap"

	"storj.io/common/sync2"
)

// Panel implements a serving of customized callbacks.
type Panel struct {
	log *zap.Logger

	url  string
	name string

	mu         sync.RWMutex
	lookup     map[string]*Button
	categories []*ButtonGroup
}

// NewPanel creates a new panel.
func NewPanel(log *zap.Logger, url, name string) *Panel {
	return &Panel{
		log:    log,
		url:    url,
		name:   name,
		lookup: map[string]*Button{},
	}
}

// ButtonGroup contains description of a collection of buttons.
type ButtonGroup struct {
	Slug    string
	Name    string
	Buttons []*Button
}

// Button defines a clickable button.
type Button struct {
	Slug string
	Name string
	Call func(progress io.Writer) error
}

// Add adds a button group to the panel.
func (panel *Panel) Add(cats ...*ButtonGroup) {
	panel.mu.Lock()
	defer panel.mu.Unlock()

	for _, cat := range cats {
		if cat.Slug == "" {
			cat.Slug = slugify(cat.Name)
		}
		for _, but := range cat.Buttons {
			but.Slug = slugify(but.Name)

			panel.lookup["/"+path.Join(cat.Slug, but.Slug)] = but
		}

		panel.categories = append(panel.categories, cat)
	}
	sort.Slice(panel.categories, func(i, k int) bool {
		return panel.categories[i].Name < panel.categories[k].Name
	})
}

// ServeHTTP serves buttons on the prefix and on
// other endpoints calls the specified call.
func (panel *Panel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	panel.mu.RLock()
	defer panel.mu.RUnlock()

	url := strings.TrimPrefix(r.URL.Path, panel.url)
	if len(url) >= len(r.URL.Path) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if url == "/" {
		err := buttonsTemplate.Execute(w, map[string]interface{}{
			"Name":       panel.name,
			"URL":        panel.url,
			"Categories": panel.categories,
		})
		if err != nil {
			panel.log.Error("buttons template failed", zap.Error(err))
		}
		return
	}

	button, ok := panel.lookup[url]
	if !ok {
		http.Error(w, "control not found", http.StatusNotFound)
		return
	}

	panel.log.Debug("calling", zap.String("url", url))

	w.Header().Add("Content-Type", "text/plain")
	w.Header().Add("Cache-Control", "no-cache")
	w.Header().Add("Cache-Control", "max-age=0")

	err := button.Call(w)
	if err != nil {
		panel.log.Error("failed to run button", zap.String("url", url), zap.Error(err))
	}
}

var buttonsTemplate = template.Must(template.New("").Parse(`<!DOCTYPE html>
<html>
<head>
<title>Control Panel</title>
<style>
.button {
	padding: 0.8rem 1rem;
	border: 1px solid #ccc;
	margin: 0.1px;
}
.button:hover {
	background: #eee;
}
</style>
</head>
<body>
	<h1>{{ .Name }} Control Panel</h1>
	{{ $url := .URL }}
	{{ range $cat := .Categories }}
	<div class="category">
	<h2>{{$cat.Name}}</h2>
	{{ range $but := $cat.Buttons }}
	<a class="button" href="{{$url}}/{{$cat.Slug}}/{{$but.Slug}}">{{.Name}}</a>
	{{ end }}
	</div>
	{{ end }}
</body>
</html>`))

// slugify converts text to a slug
func slugify(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case 'a' <= r && r <= 'z':
			return r
		case 'A' <= r && r <= 'Z':
			return unicode.ToLower(r)
		case '0' <= r && r <= '9':
			return r
		default:
			return '-'
		}
	}, s)
}

// Cycle returns button group for a cycle
func Cycle(name string, cycle *sync2.Cycle) *ButtonGroup {
	return &ButtonGroup{
		Name: name,
		Buttons: []*Button{
			{
				Name: "Trigger",
				Call: func(w io.Writer) error {
					_, _ = fmt.Fprintln(w, "Triggering")
					cycle.TriggerWait()
					_, _ = fmt.Fprintln(w, "Done")
					return nil
				},
			}, {
				Name: "Pause",
				Call: func(w io.Writer) error {
					cycle.Pause()
					_, _ = fmt.Fprintln(w, "Paused")
					return nil
				},
			}, {
				Name: "Resume",
				Call: func(w io.Writer) error {
					cycle.Restart()
					_, _ = fmt.Fprintln(w, "Resumed")
					return nil
				},
			},
		},
	}
}
