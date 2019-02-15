package dot

import (
	"bytes"
	"fmt"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/graphs"
	"sync"
)

var (
	ErrUnsupportedType = errs.Class("unsupported type error")
	EdgeFmtString      = "\"%s\"\t->\t\"%s\"\n"
)

type Graph struct {
	name string

	mu    sync.Mutex
	edges []*Edge
}

type Edge struct {
	A, B, Color, Data string
}

func New(name string) *Graph {
	return &Graph{
		name: name,
	}
}

func NewEdge(a, b, color, data string) graphs.Edge {
	edge := &Edge{
		A:     a,
		B:     b,
		Color: color,
		Data:  data,
	}
	return graphs.Edge(edge)
}

func (dot *Graph) AddEdge(edge graphs.Edge) error {
	dot.mu.Lock()
	defer dot.mu.Unlock()

	dotEdge, ok := edge.(*Edge)
	if !ok {
		return ErrUnsupportedType.New("interface type: graphs.Edge; struct type: %T", edge)
	}

	dot.edges = append(dot.edges, dotEdge)
	return nil
}

func (dot *Graph) Write(out []byte) (int, error) {
	byteCount := new(int)
	outBuf := bytes.NewBuffer(out)
	if err := dot.writeBegin(byteCount, outBuf); err != nil {
		return *byteCount, err
	}

	var edgeErrs errs.Group
	dot.mu.Lock()
	for _, edge := range dot.edges {
		// TODO: is this ok?
		i, err := edge.Write(outBuf.Bytes())
		if err != nil {
			edgeErrs.Add(err)
		} else {
			*byteCount += i
		}

	}
	dot.mu.Unlock()

	if err := dot.writeEnd(byteCount, outBuf); err != nil {
		return *byteCount, errs.Combine(edgeErrs.Err(), err)
	}
	return *byteCount, nil
}

func (dot *Graph) writeBegin(byteCount *int, out *bytes.Buffer) error {
	i, err := fmt.Fprintf(out, "digraph %s {\n", dot.name)
	if err != nil {
		return err
	}
	if byteCount != nil {
		*byteCount += i
	}
	return nil
}

func (dot *Graph) writeEnd(byteCount *int, out *bytes.Buffer) error {
	i, err := fmt.Fprintf(out, "}\n")
	if err != nil {
		return err
	}
	if byteCount != nil {
		*byteCount += i
	}
	return nil
}

func (edge *Edge) Write(out []byte) (int, error) {
	outBuf := bytes.NewBuffer(out)
	i, err := fmt.Fprintf(outBuf, EdgeFmtString, edge.A, edge.B)
	if err != nil {
		return i, err
	}
	return i, nil
}
