// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package mud

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"

	"github.com/zeebo/errs"
)

// CustomDotNode is an interface that can be implemented by a component to customize the SVG output for debugging..
type CustomDotNode interface {

	// CustomizeDotNode can replace / modify entries, which are key=value parameters of graphviz dot.
	CustomizeDotNode(tags []string) []string
}

// DotAll generates graph report of the modules in dot format.
func DotAll(w io.Writer, ball *Ball) (err error) {
	return Dot(w, ball.registry)
}

// Dot generates graph report of the modules in dot format, but only the selected components are included.
func Dot(w io.Writer, components []*Component) (err error) {
	p := func(args ...any) {
		if err != nil {
			return
		}
		_, err = fmt.Fprint(w, args...)
	}
	pf := func(format string, args ...any) {
		if err != nil {
			return
		}
		_, err = fmt.Fprintf(w, format, args...)
	}

	p("digraph G {\n")
	p("\tnode [style=filled, shape=box, fillcolor=white];\n")
	defer p("}\n")

	tagsStr := func(c *Component) string {
		var annotations []string
		for _, tag := range c.tags {
			annotations = append(annotations, fmt.Sprintf("%s", tag))
		}

		annotationStr := strings.Join(annotations, "\n")
		if len(annotationStr) > 0 {
			annotationStr = "\n" + annotationStr
		}
		return annotationStr
	}

	covered := map[reflect.Type]struct{}{}
	for _, component := range components {
		componentID := typeLabel(component.target)

		// it's an unimportant dependency. Let's hide it.
		if strings.Contains(component.Name(), "zap.Logger") {
			continue
		}
		entries := []string{"label=\"" + component.Name() + tagsStr(component) + "\""}
		if component.instance == nil {
			entries = append(entries, "color=darkgray", "fontcolor=darkgray")
		} else if component.run != nil && !component.run.started.IsZero() {
			if component.run.finished.IsZero() {
				entries = append(entries, "fillcolor=green")
			} else {
				entries = append(entries, "fillcolor=blue")
			}
		}

		for _, tag := range component.tags {
			if customize, ok := tag.(CustomDotNode); ok {
				entries = customize.CustomizeDotNode(entries)
			}
		}
		entries = append(entries, "URL=\"./"+strings.ReplaceAll(componentID, "/", "_")+"\"")

		pf("%q [%v];\n", componentID, strings.Join(entries, " "))

		covered[component.target] = struct{}{}
	}

	for _, component := range components {
		for _, dep := range component.requirements {
			if _, found := covered[dep]; !found {
				continue
			}
			componentID := typeLabel(component.target)
			pf("%q -> %q;\n", componentID, typeLabel(dep))
		}

	}
	return err

}

// GenerateComponentsGraph generates dot and svg file including the selected components.
func GenerateComponentsGraph(fileprefix string, components []*Component) error {
	var b bytes.Buffer
	if err := Dot(&b, components); err != nil {
		_, err = fmt.Fprintf(os.Stderr, "fail: %v\n", err)
		if err != nil {
			return errs.Wrap(err)
		}
	} else {
		err = os.WriteFile(fileprefix+".dot", b.Bytes(), 0644)
		if err != nil {
			return errs.Wrap(err)
		}
		output, err := exec.Command("dot", "-Tsvg", fileprefix+".dot", "-o", fileprefix+".svg").CombinedOutput()
		if err != nil {
			return errs.New("Execution of dot is failed with %s, %v", output, err)
		}
	}
	return nil
}

// MustGenerateGraph generates dot and svg files from components selected by the selector.
func MustGenerateGraph(ball *Ball, fileprefix string, selector ComponentSelector) {
	var components []*Component
	for _, c := range ball.registry {
		if selector(c) {
			components = append(components, c)
		}
	}
	err := GenerateComponentsGraph(fileprefix, components)
	if err != nil {
		panic(err)
	}
}

func typeLabel(t reflect.Type) string {
	return fullyQualifiedTypeName(t)
}
