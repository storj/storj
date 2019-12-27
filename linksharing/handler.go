// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package linksharing

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/common/ranger"
	"storj.io/common/storj"
	"storj.io/storj/lib/uplink"
)

var (
	mon = monkit.Package()
)

// HandlerConfig specifies the handler configuration
type HandlerConfig struct {
	// Uplink is the uplink used to talk to the storage network
	Uplink *uplink.Uplink

	// URLBase is the base URL of the link sharing handler. It is used
	// to construct URLs returned to clients. It should be a fully formed URL.
	URLBase string
}

// Handler implements the link sharing HTTP handler
type Handler struct {
	log     *zap.Logger
	uplink  *uplink.Uplink
	urlBase *url.URL
}

// NewHandler creates a new link sharing HTTP handler
func NewHandler(log *zap.Logger, config HandlerConfig) (*Handler, error) {
	if config.Uplink == nil {
		return nil, errs.New("uplink is required")
	}

	urlBase, err := parseURLBase(config.URLBase)
	if err != nil {
		return nil, err
	}

	return &Handler{
		log:     log,
		uplink:  config.Uplink,
		urlBase: urlBase,
	}, nil
}

// ServeHTTP handles link sharing requests
func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// serveHTTP handles the request in full. the error that is returned can
	// be ignored since it was only added to facilitate monitoring.
	_ = handler.serveHTTP(w, r)
}

func (handler *Handler) serveHTTP(w http.ResponseWriter, r *http.Request) (err error) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(&err)

	locationOnly := false

	switch r.Method {
	case http.MethodHead:
		locationOnly = true
	case http.MethodGet:
	default:
		err = errs.New("method not allowed")
		http.Error(w, err.Error(), http.StatusMethodNotAllowed)
		return err
	}

	scope, bucket, unencPath, err := parseRequestPath(r.URL.Path)
	if err != nil {
		err = fmt.Errorf("invalid request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	p, err := handler.uplink.OpenProject(ctx, scope.SatelliteAddr, scope.APIKey)
	if err != nil {
		handler.handleUplinkErr(w, "open project", err)
		return err
	}
	defer func() {
		if err := p.Close(); err != nil {
			handler.log.With(zap.Error(err)).Warn("unable to close project")
		}
	}()

	b, err := p.OpenBucket(ctx, bucket, scope.EncryptionAccess)
	if err != nil {
		handler.handleUplinkErr(w, "open bucket", err)
		return err
	}
	defer func() {
		if err := b.Close(); err != nil {
			handler.log.With(zap.Error(err)).Warn("unable to close bucket")
		}
	}()

	o, err := b.OpenObject(ctx, unencPath)
	if err != nil {
		handler.handleUplinkErr(w, "open object", err)
		return err
	}
	defer func() {
		if err := o.Close(); err != nil {
			handler.log.With(zap.Error(err)).Warn("unable to close object")
		}
	}()

	if locationOnly {
		location := makeLocation(handler.urlBase, r.URL.Path)
		http.Redirect(w, r, location, http.StatusFound)
		return nil
	}

	ranger.ServeContent(ctx, w, r, unencPath, o.Meta.Modified, newObjectRanger(o))
	return nil
}

func (handler *Handler) handleUplinkErr(w http.ResponseWriter, action string, err error) {
	switch {
	case storj.ErrBucketNotFound.Has(err):
		http.Error(w, "bucket not found", http.StatusNotFound)
	case storj.ErrObjectNotFound.Has(err):
		http.Error(w, "object not found", http.StatusNotFound)
	default:
		handler.log.Error("unable to handle request", zap.String("action", action), zap.Error(err))
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
			return nil, "", "", errs.New("missing scope")
		}
		return nil, "", "", errs.New("missing bucket")
	case 2:
		return nil, "", "", errs.New("missing bucket path")
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

func (ranger *objectRanger) Size() int64 {
	return ranger.o.Meta.Size
}

func (ranger *objectRanger) Range(ctx context.Context, offset, length int64) (_ io.ReadCloser, err error) {
	defer mon.Task()(&ctx)(&err)
	return ranger.o.DownloadRange(ctx, offset, length)
}

func parseURLBase(s string) (*url.URL, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, errs.New("URL base must be http:// or https://")
	case u.Host == "":
		return nil, errs.New("URL base must contain host")
	case u.User != nil:
		return nil, errs.New("URL base must not contain user info")
	case u.RawQuery != "":
		return nil, errs.New("URL base must not contain query values")
	case u.Fragment != "":
		return nil, errs.New("URL base must not contain a fragment")
	}
	return u, nil
}

func makeLocation(base *url.URL, reqPath string) string {
	location := *base
	location.Path = path.Join(location.Path, reqPath)
	return location.String()
}
