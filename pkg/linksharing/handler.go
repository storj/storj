// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package linksharing

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/lib/uplink"
	"storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storj"
)

var (
	mon = monkit.Package()
)

// Handler implements the link sharing HTTP handler
type Handler struct {
	log    *zap.Logger
	uplink *uplink.Uplink
}

// NewHandler creates a new link sharing HTTP handler
func NewHandler(log *zap.Logger, uplink *uplink.Uplink) *Handler {
	return &Handler{
		log:    log,
		uplink: uplink,
	}
}

// ServeHTTP handles link sharing requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// serveHTTP handles the request in full. the error that is returned can
	// be ignored since it was only added to facilitate monitoring.
	_ = h.serveHTTP(w, r)
}

func (h *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(&err)

	if r.Method != http.MethodGet {
		err = errors.New("method not allowed")
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return err
	}

	scope, bucket, unencPath, err := parseRequestPath(r.URL.Path)
	if err != nil {
		err = fmt.Errorf("invalid request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p, err := h.uplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		h.handleUplinkErr(w, "open project", err)
		return err
	}
	defer func() {
		if err := p.Close(); err != nil {
			h.log.With(zap.Error(err)).Warn("unable to close project")
		}
	}()

	b, err := p.OpenBucket(ctx, bucket, scope.EncryptionAccess)
	if err != nil {
		h.handleUplinkErr(w, "open bucket", err)
		return err
	}
	defer func() {
		if err := b.Close(); err != nil {
			h.log.With(zap.Error(err)).Warn("unable to close bucket")
		}
	}()

	o, err := b.OpenObject(ctx, unencPath)
	if err != nil {
		h.handleUplinkErr(w, "open object", err)
		return err
	}
	defer func() {
		if err := o.Close(); err != nil {
			h.log.With(zap.Error(err)).Warn("unable to close object")
		}
	}()

	ranger.ServeContent(ctx, w, r, unencPath, o.Meta.Modified, newObjectRanger(o))
	return nil
}

func (h *Handler) handleUplinkErr(w http.ResponseWriter, action string, err error) {
	switch {
	case storj.ErrBucketNotFound.Has(err):
		http.Error(w, "bucket not found", http.StatusNotFound)
	case storj.ErrObjectNotFound.Has(err):
		http.Error(w, "object not found", http.StatusNotFound)
	default:
		h.log.Error("unable to handle request", zap.String("action", action), zap.Error(err))
		http.Error(w, "unable to handle request", http.StatusInternalServerError)
	}
}

func parseRequestPath(p string) (*uplink.Scope, string, string, error) {
	// Drop the leading slash, if necessary
	p = strings.TrimPrefix(p, "/")

	// Split the request path
	segments := strings.SplitN(p, "/", 3)
	switch len(segments) {
	case 1:
		if segments[0] == "" {
			return nil, "", "", errors.New("missing scope")
		}
		return nil, "", "", errors.New("missing bucket")
	case 2:
		return nil, "", "", errors.New("missing bucket path")
	}
	scopeb58 := segments[0]
	bucket := segments[1]
	unencPath := segments[2]

	scope, err := uplink.ParseScope(scopeb58)
	if err != nil {
		return nil, "", "", err
	}
	return scope, bucket, unencPath, nil
}

type objectRanger struct {
	o *uplink.Object
}

func newObjectRanger(o *uplink.Object) ranger.Ranger {
	return &objectRanger{
		o: o,
	}
}

func (r *objectRanger) Size() int64 {
	return r.o.Meta.Size
}

func (r *objectRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	return r.o.DownloadRange(ctx, offset, length)
}
