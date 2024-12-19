// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package debug

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"os/exec"
	"runtime/pprof"
	"sort"
	"strings"

	"github.com/spacemonkeygo/monkit/v3"

	"storj.io/storj/shared/mud"
)

// ModuleGraph is a debug extension to visualize the module graph.
type ModuleGraph struct {
	ball *mud.Ball
}

// Description implements the debug.Extension interface.
func (m ModuleGraph) Description() string {
	return "Actual module graph"
}

// Path implements the debug.Extension interface.
func (m ModuleGraph) Path() string {
	return "/mud/"
}

//go:embed component.html
var componentTemplate string

// Handler is the HTTP handler for the module graph.
func (m ModuleGraph) Handler(writer http.ResponseWriter, request *http.Request) {
	c := strings.TrimPrefix(request.RequestURI, "/mud/")
	c = strings.Split(c, "?")[0]
	if c == "" {
		m.GenerateSVG(writer, request)
		return
	}
	selected := m.findComponent(c)
	if selected == nil {
		writer.WriteHeader(http.StatusNotFound)
		_, _ = writer.Write([]byte("not found"))
		return
	}
	out, err := m.componentPage(selected)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte("internal error: " + err.Error()))
	}
	_, _ = writer.Write([]byte(out))
}

func (m ModuleGraph) componentPage(c *mud.Component) (string, error) {
	raw, err := json.MarshalIndent(c.Instance(), "", "   ")
	if err != nil {
		return "", err
	}

	tpl, err := htmltemplate.New("component").Parse(componentTemplate)
	if err != nil {
		return "", err
	}

	res := bytes.NewBuffer([]byte{})

	var metrics []string
	monkit.Default.Stats(func(key monkit.SeriesKey, field string, val float64) {
		id := ""
		if strings.Contains(key.Tags.Get("name"), "*") {
			id += "*"
		}
		id += key.Tags.Get("scope") + "."
		id += strings.Trim(strings.Split(key.Tags.Get("name"), ".")[0], "*()")
		if id != c.ID() {
			return
		}
		metrics = append(metrics, fmt.Sprintf("%s %s %f", key, field, val))
	})
	sort.Strings(metrics)

	gr := bytes.NewBuffer([]byte{})

	_ = pprof.Lookup("goroutine").WriteTo(gr, 1)
	filtered := ""
	matched := false
	buffer := ""
	for _, line := range strings.Split(gr.String(), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			if matched {
				filtered += buffer + "\n"
			}
			matched = false
			buffer = ""
		}
		target := c.GetTarget().String()
		v := ""
		if strings.HasPrefix(target, "*") {
			v = "*"
			target = target[1:]
		}
		parts := strings.Split(target, ".")
		pattern := ""
		if len(parts) >= 2 {
			pattern = fmt.Sprintf("%s.(%s%s)", parts[len(parts)-2], v, parts[len(parts)-1])
		}
		if strings.Contains(line, pattern) {
			matched = true
		}
		buffer += line + "\n"
	}

	err = tpl.Execute(res, map[string]interface{}{
		"Json":    string(raw),
		"Slug":    strings.ReplaceAll(c.ID(), "/", "_"),
		"Metrics": metrics,
		"Gr":      filtered,
	})
	if err != nil {
		return "", err
	}
	return res.String(), nil
}

func (m ModuleGraph) findComponent(c string) *mud.Component {
	var selected *mud.Component
	_ = mud.ForEach(m.ball, func(component *mud.Component) error {
		if strings.ReplaceAll(component.ID(), "/", "_") == c {
			selected = component
		}
		return nil
	}, mud.All)
	return selected
}

// GenerateSVG generates a SVG representation of the module (sub) graph.
func (m ModuleGraph) GenerateSVG(writer http.ResponseWriter, request *http.Request) {
	out := bytes.NewBuffer([]byte{})
	var err error
	selected := request.URL.Query().Get("root")
	if selected != "" {
		components := mud.FindSelectedWithDependencies(m.ball, func(component *mud.Component) bool {
			return strings.ReplaceAll(component.ID(), "/", "_") == selected
		})
		err = mud.Dot(out, components)
	} else if request.URL.Query().Get("all") != "" {
		err = mud.DotAll(out, m.ball)
	} else {
		err = mud.Dot(out, mud.Find(m.ball, func(c *mud.Component) bool {
			return c.Instance() != nil
		}))
	}

	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write([]byte(err.Error()))
		return
	}

	if request.URL.Query().Get("type") == "dot" {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write(out.Bytes())
	}
	svgBuffer := bytes.NewBuffer([]byte{})
	command := exec.Command("dot", "-Tsvg")
	command.Stdin = out
	command.Stdout = svgBuffer
	command.Stderr = svgBuffer
	err = command.Run()
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		_, _ = writer.Write(svgBuffer.Bytes())
		_, _ = writer.Write([]byte(err.Error()))
		return
	}
	writer.Header().Add("Content-Type", "image/svg+xml")
	writer.WriteHeader(http.StatusOK)
	result := svgBuffer.String()
	result = strings.ReplaceAll(result, "xlink:title", "target=\"_top\" xlink:title")
	_, _ = writer.Write([]byte(result))
}
