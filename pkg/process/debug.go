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

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/present"
	"go.uber.org/zap"

	"storj.io/storj/internal/version/checker"
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

	mux.Handle("/version/", http.StripPrefix("/version", checker.NewDebugHandler(logger.Named("version"))))
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
		case '0' <= r && r <= '9':
			return r
		default:
			return '_'
		}
	}, val)
}

func prometheus(w http.ResponseWriter, r *http.Request) {
	// writes https://prometheus.io/docs/instrumenting/exposition_formats/
	// (https://prometheus.io/docs/concepts/metric_types/)
	monkit.Default.Stats(func(series monkit.Series, val float64) {
		measurement := sanitize(series.Measurement)
		var metrics []string
		for tag, tagVal := range series.Tags.All() {
			metric := sanitize(tag) + "=\"" + sanitize(tagVal) + "\""
			metrics = append(metrics, metric)
		}
		fieldMetric := "field=\"" + sanitize(series.Field) + "\""
		metrics = append(metrics, fieldMetric)

		_, _ = fmt.Fprintf(w, "# TYPE %s gauge\n%s{"+
			strings.Join(metrics, ",")+"} %g\n", measurement, measurement, val)
	})
}
