// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"fmt"
	"net"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/present"

	"storj.io/fork/net/http"
	"storj.io/fork/net/http/pprof"
)

var (
	debugAddr = flag.String("debug.addr", "localhost:0", "address to listen on for debug endpoints")
)

func init() {
	// zero out the http.DefaultServeMux net/http/pprof so unhelpfully
	// side-effected.
	*http.DefaultServeMux = http.ServeMux{}
}

type monkitHTTPHandler struct {
	Registry *monkit.Registry
}

// monkitHTTP makes an http.Handler out of a Registry. It serves paths using this
// package's FromRequest request router. Usually monkitHTTP is called with the
// Default registry.
func monkitHTTP(r *monkit.Registry) http.Handler {
	return monkitHTTPHandler{Registry: r}
}

func (h monkitHTTPHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	p, contentType, err := present.FromRequest(h.Registry, req.URL.Path, req.URL.Query())
	if err != nil {
		// no good way to tell errors apart here without forking monkit :(
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", contentType)
	p(w)
}

func initDebug(logger *zap.Logger, r *monkit.Registry) (err error) {
	var mux http.ServeMux
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/mon/", http.StripPrefix("/mon", monkitHTTP(r)))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "OK")
	})
	ln, err := net.Listen("tcp", *debugAddr)
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
