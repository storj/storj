package dot

import (
	"bytes"
	"fmt"
	"github.com/zeebo/errs"
	"sync"
)

type Graph struct {
	name string

	mu    sync.Mutex
	edges []*Edge
	out   *bytes.Buffer
}

type Edge struct {
}

func New(name string) *Graph {
	return &Graph{
		name: name,
		out:  new(bytes.Buffer),
	}
}

func (dot *Graph) AddEdge(edge *Edge) {
	dot.mu.Lock()
	defer dot.mu.Unlock()

	dot.edges = append(dot.edges, edge)
}

func (dot *Graph) Write() (int, error) {
	byteCount := new(int)
	out := new(bytes.Buffer)
	if err := dot.writeBegin(byteCount); err != nil {
		return *byteCount, err
	}

	var edgeErrs errs.Group{}
	dot.mu.Lock()
	for _, e := range dot.edges {
		i, err := e.Write()
		if err != nil {
			edgeErrs.Add(err)
		} else {
			*byteCount += i
		}

	}
	dot.mu.Unlock()

	if err := dot.writeEnd(byteCount); err != nil {
		return *byteCount, errs.Combine(edgeErrs.Err(), err)
	}
}

func (dot *Graph) writeBegin(byteCount *int) error {
	i, err := fmt.Fprintf(dot.out, "digraph %s {\n", dot.name)
	if err != nil {
		return err
	}
	if byteCount != nil {
		*byteCount += i
	}
	return nil
}

func (dot *Graph) writeEnd(byteCount *int) error {
	i, err := fmt.Fprintf(dot.out, "}\n")
	if err != nil {
		return err
	}
	if byteCount != nil {
		*byteCount += i
	}
	return nil
}

func (edge *Edge) Write() (int, error) {
	i, err := fmt.Fprintf(edge.out, EdgeStringFmt, a, b)
}
