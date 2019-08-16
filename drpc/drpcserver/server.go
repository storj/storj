// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcserver

import (
	"context"
	"fmt"
	"io"
	"reflect"

	"github.com/zeebo/errs"
	"storj.io/storj/drpc"
	"storj.io/storj/drpc/drpcmanager"
	"storj.io/storj/drpc/drpcstream"
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

	switch mt := reflect.TypeOf(method); {
	// unitary input, unitary output
	case mt.NumOut() == 2:
		data.in1 = mt.In(2)
		if !data.in1.Implements(messageType) {
			panicf("input argument not a drpc message: %v", data.in1)
		}

	// unitary input, stream output
	case mt.NumIn() == 3:
		data.in1 = mt.In(1)
		if !data.in1.Implements(messageType) {
			panicf("input argument not a drpc message: %v", data.in1)
		}
		data.in2 = streamType

	// stream input
	case mt.NumIn() == 2:
		data.in1 = streamType

	// code gen bug?
	default:
		panicf("unknown method type: %v", mt)
	}

	s.rpcs[rpc] = data
}

func (s *Server) Manage(ctx context.Context, rw io.ReadWriter) error {
	return drpcmanager.New(rw, s).Run(ctx)
}

func (s *Server) Handle(stream *drpcstream.Stream, rpc string) error {
	err := s.doHandle(stream, rpc)
	if err != nil {
		stream.Sig().Set(err)
	}
	return errs.Combine(err, stream.Close())
}

func (s *Server) doHandle(stream *drpcstream.Stream, rpc string) error {
	data, ok := s.rpcs[rpc]
	if !ok {
		return drpc.ProtocolError.New("unknown rpc: %q", rpc)
	}

	in := interface{}(stream)
	if data.in1 != streamType {
		msg := reflect.New(data.in1.Elem()).Interface().(drpc.Message)
		if err := stream.MsgRecv(msg); err != nil {
			return err
		}
		in = msg
	}

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	out, err := data.handler(data.srv, ctx, in, stream)
	switch {
	case err != nil:
		return err
	case out != nil:
		return stream.MsgSend(out)
	default:
		return nil
	}
}
