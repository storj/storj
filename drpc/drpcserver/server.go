// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcserver

import (
	"fmt"
	"io"
	"reflect"

	"storj.io/storj/drpc"
)

type Server struct {
	rpcs map[string]rpcData
}

func New() *Server {
	return &Server{
		rpcs: make(map[string]rpcData),
	}
}

func panicf(format string, a ...interface{}) {
	panic(fmt.Sprintf(format, a...))
}

var (
	streamType  = reflect.TypeOf((*drpc.Stream)(nil)).Elem()
	messageType = reflect.TypeOf((*drpc.Message)(nil)).Elem()
)

type rpcData struct {
	srv     interface{}
	handler drpc.Handler
	in1     reflect.Type
	in2     reflect.Type
}

func (s *Server) Register(srv interface{}, desc drpc.Description) {
	n := desc.NumMethods()
	for i := 0; i < n; i++ {
		rpc, handler, method, ok := desc.Method(i)
		if !ok {
			panicf("description returned not ok for method %d", i)
		}
		s.registerOne(srv, rpc, handler, method)
	}
}

func (s *Server) registerOne(srv interface{}, rpc string, handler drpc.Handler, method interface{}) {
	if _, ok := s.rpcs[rpc]; ok {
		panicf("rpc already registered for %q", rpc)
	}
	data := rpcData{srv: srv, handler: handler}
	mt := reflect.TypeOf(method)
	switch {
	case mt.NumOut() == 2: // unitary input, unitary output
		data.in1 = mt.In(2)
		if !data.in1.Implements(messageType) {
			panicf("input argument not a drpc message: %v", data.in1)
		}
	case mt.NumIn() == 3: // unitary input, stream output
		data.in1 = mt.In(1)
		if !data.in1.Implements(messageType) {
			panicf("input argument not a drpc message: %v", data.in1)
		}
		data.in2 = streamType
	case mt.NumIn() == 2: // stream input
		data.in1 = streamType
	default:
		panicf("unknown method type: %v", mt)
	}
	s.rpcs[rpc] = data
}

// TODO(jeff): maybe there's a way to share more of this dispatch code

func (s *Server) Handle(rw io.ReadWriter) error {
	return newSession(rw, s.rpcs).Run()
}
