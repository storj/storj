// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

// Domains is an api controller that exposes all domains functionality.
type Domains struct {
	log     *zap.Logger
	service *console.Service
}

// NewDomains is a constructor for Domains controller.
func NewDomains(log *zap.Logger, service *console.Service) *Domains {
	return &Domains{
		log:     log,
		service: service,
	}
}

// CheckDNSRecords checks DNS records by provided domain.
func (d *Domains) CheckDNSRecords(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	_, err = console.GetUser(ctx)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	domain := r.URL.Query().Get("domain")
	if domain == "" {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing 'domain' query parameter"))
		return
	}

	parts := strings.Split(domain, ".")
	if len(parts) < 2 {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid domain format"))
	}

	var payload struct {
		CNAME string   `json:"cname"`
		TXT   []string `json:"txt"`
	}

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	cname, err := net.LookupCNAME(domain)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusNotFound, err)
		return
	}

	txt, err := net.LookupTXT("txt-" + domain)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusNotFound, err)
		return
	}

	if payload.CNAME != cname {
		d.serveJSONError(ctx, w, http.StatusConflict, errs.New("CNAME is not correct. Got: %s, Expected: %s", cname, payload.CNAME))
		return
	}

	sort.Strings(txt)
	sort.Strings(payload.TXT)

	equal := reflect.DeepEqual(txt, payload.TXT)
	if !equal {
		d.serveJSONError(ctx, w, http.StatusConflict, errs.New("TXT is not correct. Got: %v, Expected %v", txt, payload.TXT))
	}
}

// serveJSONError writes JSON error to response output stream.
func (d *Domains) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, d.log, w, status, err)
}
