// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

var (
	// ErrDomainsAPI - console domains api error type.
	ErrDomainsAPI = errs.Class("console domains")
)

// maxSubdomainLength is the maximum length of a subdomain, according to RFC 1035.
const maxSubdomainLength = 253

// Domains is an api controller that exposes all domains functionality.
type Domains struct {
	log     *zap.Logger
	service *console.Service

	domainsPageEnabled bool
}

// NewDomains is a constructor for Domains controller.
func NewDomains(log *zap.Logger, service *console.Service, domainsPageEnabled bool) *Domains {
	return &Domains{
		log:                log,
		service:            service,
		domainsPageEnabled: domainsPageEnabled,
	}
}

// CreateDomain creates new domain for a given project.
func (d *Domains) CreateDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	if !d.domainsPageEnabled {
		d.serveJSONError(ctx, w, http.StatusNotImplemented, errs.New("domains page is disabled"))
		return
	}

	idParam, ok := mux.Vars(r)["projectID"]
	if !ok {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing projectID route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var payload console.Domain
	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if utf8.RuneCountInString(payload.Subdomain) < 3 || utf8.RuneCountInString(payload.Subdomain) > maxSubdomainLength {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("subdomain length must be between 3 and %d characters", maxSubdomainLength))
		return
	}

	if utf8.RuneCountInString(payload.Prefix) < 3 {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("prefix length must be at least 3 characters"))
	}

	err = d.service.ValidateLongFormInputLengths(&payload.Prefix)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("prefix is too long: %w", err))
		return
	}

	payload.ProjectPublicID = projectID

	_, err = d.service.CreateDomain(ctx, payload)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			d.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if console.ErrNotPaidTier.Has(err) {
			d.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}
		if console.ErrSubdomainAlreadyExists.Has(err) {
			d.serveJSONError(ctx, w, http.StatusConflict, err)
			return
		}

		d.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// DeleteDomain deletes a domain for a given project.
func (d *Domains) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	if !d.domainsPageEnabled {
		d.serveJSONError(ctx, w, http.StatusNotImplemented, errs.New("domains page is disabled"))
		return
	}

	idParam, ok := mux.Vars(r)["projectID"]
	if !ok {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing projectID route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	subdomain := strings.TrimSpace(string(bodyBytes))

	if utf8.RuneCountInString(subdomain) < 3 || utf8.RuneCountInString(subdomain) > maxSubdomainLength {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("subdomain length must be between 3 and %d characters", maxSubdomainLength))
		return
	}

	err = d.service.DeleteDomain(ctx, projectID, subdomain)
	if err != nil {
		switch {
		case console.ErrUnauthorized.Has(err):
			d.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		case console.ErrNoSubdomain.Has(err):
			d.serveJSONError(ctx, w, http.StatusNotFound, err)
		case console.ErrForbidden.Has(err):
			d.serveJSONError(ctx, w, http.StatusForbidden, err)
		default:
			d.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		}
	}
}

// GetProjectDomains returns paged domains by project ID.
func (d *Domains) GetProjectDomains(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	if !d.domainsPageEnabled {
		d.serveJSONError(ctx, w, http.StatusNotImplemented, errs.New("domains page is disabled"))
		return
	}

	idParam, ok := mux.Vars(r)["projectID"]
	if !ok {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing projectID route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	query := r.URL.Query()

	limitParam := query.Get("limit")
	if limitParam == "" {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'limit' can't be empty"))
		return
	}

	limit, err := strconv.ParseUint(limitParam, 10, 32)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	pageParam := query.Get("page")
	if pageParam == "" {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'page' can't be empty"))
		return
	}

	page, err := strconv.ParseUint(pageParam, 10, 32)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	orderParam := query.Get("order")
	if orderParam == "" {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'order' can't be empty"))
		return
	}

	order, err := strconv.ParseUint(orderParam, 10, 32)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	orderDirectionParam := query.Get("orderDirection")
	if orderDirectionParam == "" {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("parameter 'orderDirection' can't be empty"))
		return
	}

	orderDirection, err := strconv.ParseUint(orderDirectionParam, 10, 32)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	searchString := query.Get("search")

	cursor := console.DomainCursor{
		Search:         searchString,
		Limit:          uint(limit),
		Page:           uint(page),
		Order:          console.DomainOrder(order),
		OrderDirection: console.OrderDirection(orderDirection),
	}

	domainsPage, err := d.service.ListDomains(ctx, projectID, cursor)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			d.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		d.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(domainsPage)
	if err != nil {
		d.log.Error("failed to write json domains page response", zap.Error(ErrDomainsAPI.Wrap(err)))
	}
}

// GetProjectAllDomainNames returns all domain names by project ID.
func (d *Domains) GetProjectAllDomainNames(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	if !d.domainsPageEnabled {
		d.serveJSONError(ctx, w, http.StatusNotImplemented, errs.New("domains page is disabled"))
		return
	}

	idParam, ok := mux.Vars(r)["projectID"]
	if !ok {
		d.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing projectID route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		d.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	names, err := d.service.GetAllDomainNames(ctx, projectID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			d.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		d.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(names)
	if err != nil {
		d.log.Error("failed to write json all domain names response", zap.Error(ErrDomainsAPI.Wrap(err)))
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

	if !d.domainsPageEnabled {
		d.serveJSONError(ctx, w, http.StatusNotImplemented, errs.New("domains page is disabled"))
		return
	}

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

	cname, err := (&net.Resolver{}).LookupCNAME(ctx, domain)
	if err != nil {
		d.sendResponse(w, checkDNSRecordsResponse{IsVerifyError: true})
		return
	}

	txt, err := (&net.Resolver{}).LookupTXT(ctx, "txt-"+domain)
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
