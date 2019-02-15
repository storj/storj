package graphs

import "io"

type Edge interface {
	io.Writer
}

type Graph interface {
	AddEdge(Edge) error
	io.Writer
}
