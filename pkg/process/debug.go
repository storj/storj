// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"context"
	"net"
	"net/http"
	"net/http/pprof"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/present"
)

func init() {
	// zero out the http.DefaultServeMux net/http/pprof so unhelpfully
	// side-effected.
	*http.DefaultServeMux = http.ServeMux{}
}

func initDebug(ctx context.Context, logger *zap.Logger, r *monkit.Registry) (
	err error) {
	var mux http.ServeMux
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/mon/", http.StripPrefix("/mon", present.HTTP(r)))
	ln, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}
	go func() {
		err := (&http.Server{Handler: &mux}).Serve(ln)
		if err != nil {
			logger.Error("debug server died", zap.Error(err))
		}
	}()
	return nil
}
