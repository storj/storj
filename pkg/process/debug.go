// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"strings"

	"go.uber.org/zap"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"
	"gopkg.in/spacemonkeygo/monkit.v2/present"

	"storj.io/storj/internal/version"
)

var (
	debugAddr = flag.String("debug.addr", "127.0.0.1:0", "address to listen on for debug endpoints")
)

func init() {
	// zero out the http.DefaultServeMux net/http/pprof so unhelpfully
	// side-effected.
	*http.DefaultServeMux = http.ServeMux{}
}

func initDebug(logger *zap.Logger, r *monkit.Registry) (err error) {
	var mux http.ServeMux
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	mux.Handle("/version/", http.StripPrefix("/version", http.HandlerFunc(version.DebugHandler)))
	mux.Handle("/mon/", http.StripPrefix("/mon", present.HTTP(r)))
	mux.HandleFunc("/metrics", prometheus)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintln(w, "OK")
	})
	if *debugAddr != "" {
		ln, err := net.Listen("tcp", *debugAddr)
		if err != nil {
			return err
		}
		go func() {
			logger.Debug(fmt.Sprintf("debug server listening on %s", ln.Addr().String()))
			err := (&http.Server{Handler: &mux}).Serve(ln)
			if err != nil {
				logger.Error("debug server died", zap.Error(err))
			}
		}()
	}
	return nil
}

func sanitize(val string) string {
	// https://prometheus.io/docs/concepts/data_model/
	// specifies all metric names must match [a-zA-Z_:][a-zA-Z0-9_:]*
	// Note: The colons are reserved for user defined recording rules.
	// They should not be used by exporters or direct instrumentation.
	if '0' <= val[0] && val[0] <= '9' {
		val = "_" + val
	}
	return strings.Map(func(r rune) rune {
		switch {
		case 'a' <= r && r <= 'z':
			return r
		case 'A' <= r && r <= 'Z':
			return r
		default:
			return '_'
		}
	}, val)
}

func prometheus(w http.ResponseWriter, r *http.Request) {
	// writes https://prometheus.io/docs/instrumenting/exposition_formats/
	// TODO(jt): deeper monkit integration so we can expose prometheus types
	// (https://prometheus.io/docs/concepts/metric_types/)
	monkit.Default.Stats(func(name string, val float64) {
		metric := sanitize(name)
		_, _ = fmt.Fprintf(w, "# HELP %s %s\n%s %g\n",
			metric, strings.ReplaceAll(name, "\n", " "),
			metric, val)
	})
}
