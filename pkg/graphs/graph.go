package graph

import "io"

type Edge interface {
	String() string
	io.Writer
}

type Graph interface {
	AddEdge(Edge)
	io.Writer
}

