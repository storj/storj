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

var (
	// ErrDomainsAPI - console domains api error type.
	ErrDomainsAPI = errs.Class("console domains")
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

type checkDNSRecordsResponse struct {
	IsSuccess     bool     `json:"isSuccess"`
	IsVerifyError bool     `json:"isVerifyError"`
	ExpectedCNAME string   `json:"expectedCNAME"`
	ExpectedTXT   []string `json:"expectedTXT"`
	GotCNAME      string   `json:"gotCNAME"`
	GotTXT        []string `json:"gotTXT"`
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
		d.sendResponse(w, checkDNSRecordsResponse{IsVerifyError: true})
		return
	}

	txt, err := net.LookupTXT("txt-" + domain)
	if err != nil {
		d.sendResponse(w, checkDNSRecordsResponse{IsVerifyError: true})
		return
	}

	if payload.CNAME != cname {
		d.sendResponse(w, checkDNSRecordsResponse{
			ExpectedCNAME: payload.CNAME,
			GotCNAME:      cname,
		})
		return
	}

	sort.Strings(txt)
	sort.Strings(payload.TXT)

	equal := reflect.DeepEqual(txt, payload.TXT)
	if !equal {
		d.sendResponse(w, checkDNSRecordsResponse{
			ExpectedTXT: payload.TXT,
			GotTXT:      txt,
		})
		return
	}

	d.sendResponse(w, checkDNSRecordsResponse{IsSuccess: true})
}

func (d *Domains) sendResponse(w http.ResponseWriter, response checkDNSRecordsResponse) {
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		d.log.Error("failed to write json check DNS records response", zap.Error(ErrDomainsAPI.Wrap(err)))
	}
}

// serveJSONError writes JSON error to response output stream.
func (d *Domains) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, d.log, w, status, err)
}
