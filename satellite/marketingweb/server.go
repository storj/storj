// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/satellite/rewards"
)

// Error is satellite marketing error type
var Error = errs.Class("satellite marketing error")

// Config contains configuration for marketingweb server
type Config struct {
	Address   string `help:"server address of the marketing Admin GUI" default:"127.0.0.1:8090"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents marketing offersweb server
type Server struct {
	log         *zap.Logger
	config      Config
	listener    net.Listener
	server      http.Server
	db          rewards.DB
	templateDir string
	templates   struct {
		home          *template.Template
		pageNotFound  *template.Template
		internalError *template.Template
		badRequest    *template.Template
	}
}

// commonPages returns templates that are required for all routes.
func (s *Server) commonPages() []string {
	return []string{
		filepath.Join(s.templateDir, "base.html"),
		filepath.Join(s.templateDir, "index.html"),
		filepath.Join(s.templateDir, "banner.html"),
		filepath.Join(s.templateDir, "logo.html"),
	}
}

// NewServer creates new instance of offersweb server
func NewServer(logger *zap.Logger, config Config, db rewards.DB, listener net.Listener) (*Server, error) {
	s := &Server{
		log:      logger,
		config:   config,
		listener: listener,
		db:       db,
	}

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", s.listener.Addr().String())
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(s.config.StaticDir)))
	mux := mux.NewRouter()
	if s.config.StaticDir != "" {
		mux.HandleFunc("/", s.GetOffers)
		mux.PathPrefix("/static/").Handler(fs)
		mux.HandleFunc("/create/{offer_type}", s.CreateOffer)
		mux.HandleFunc("/stop/{offer_id}", s.StopOffer)
	}
	s.server.Handler = mux

	s.templateDir = filepath.Join(s.config.StaticDir, "pages")

	if err := s.parseTemplates(); err != nil {
		return nil, Error.Wrap(err)
	}

	return s, nil
}

// GetOffers renders the tables for free credits and referral offers to the UI
func (s *Server) GetOffers(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveNotFound(w, req)
		return
	}

	offers, err := s.db.ListAll(req.Context())
	if err != nil {
		s.log.Error("failed to retrieve all offers", zap.Error(err))
		s.serveInternalError(w, req, err)
		return
	}

	if err := s.templates.home.ExecuteTemplate(w, "base", offers.OrganizeOffersByType()); err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
		s.serveInternalError(w, req, err)
	}
}

// parseTemplates parses and stores all templates in server
func (s *Server) parseTemplates() (err error) {
	homeFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "home.html"),
		filepath.Join(s.templateDir, "referral-offers.html"),
		filepath.Join(s.templateDir, "referral-offers-modal.html"),
		filepath.Join(s.templateDir, "free-offers.html"),
		filepath.Join(s.templateDir, "free-offers-modal.html"),
		filepath.Join(s.templateDir, "partner-offers.html"),
		filepath.Join(s.templateDir, "partner-offers-modal.html"),
		filepath.Join(s.templateDir, "stop-free-credit.html"),
		filepath.Join(s.templateDir, "stop-referral-offer.html"),
	)

	pageNotFoundFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "page-not-found.html"),
	)

	internalErrorFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "internal-server-error.html"),
	)

	badRequestFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "err.html"),
	)

	s.templates.home, err = template.New("landingPage").Funcs(template.FuncMap{
		"isEmpty": rewards.Offer.IsEmpty,
	}).ParseFiles(homeFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	s.templates.pageNotFound, err = template.New("page-not-found").ParseFiles(pageNotFoundFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	s.templates.internalError, err = template.New("internal-server-error").ParseFiles(internalErrorFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	s.templates.badRequest, err = template.New("bad-request-error").ParseFiles(badRequestFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// CreateOffer handles requests to create new offers.
func (s *Server) CreateOffer(w http.ResponseWriter, req *http.Request) {
	offer, err := parseOfferForm(w, req)
	if err != nil {
		s.log.Error("failed to convert form to struct", zap.Error(err))
		s.serveBadRequest(w, req, err)
		return
	}

	offer.Status = rewards.Active
	offerType := mux.Vars(req)["offer_type"]

	switch offerType {
	case "referral-offer":
		offer.Type = rewards.Referral
	case "free-credit":
		offer.Type = rewards.FreeCredit
	default:
		err := errs.New("response status %d : invalid offer type", http.StatusBadRequest)
		s.serveBadRequest(w, req, err)
		return
	}

	if _, err := s.db.Create(req.Context(), &offer); err != nil {
		s.log.Error("failed to insert new offer", zap.Error(err))
		s.serveBadRequest(w, req, err)
		return
	}

	http.Redirect(w, req, "/", http.StatusSeeOther)
}

// StopOffer expires the current offer and replaces it with the default offer.
func (s *Server) StopOffer(w http.ResponseWriter, req *http.Request) {
	offerID, err := strconv.Atoi(mux.Vars(req)["offer_id"])
	if err != nil {
		s.log.Error("failed to parse offer id", zap.Error(err))
		s.serveBadRequest(w, req, err)
		return
	}

	if err := s.db.Finish(req.Context(), offerID); err != nil {
		s.log.Error("failed to stop offer", zap.Error(err))
		s.serveInternalError(w, req, err)
		return
	}

	http.Redirect(w, req, "/", http.StatusSeeOther)
}

// serveNotFound handles 404 errors and defaults to 500 if template parsing fails.
func (s *Server) serveNotFound(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusNotFound)

	err := s.templates.pageNotFound.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

// serveInternalError handles 500 errors and renders err to the internal-server-error template.
func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request, errMsg error) {
	w.WriteHeader(http.StatusInternalServerError)

	if err := s.templates.internalError.ExecuteTemplate(w, "base", errMsg); err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

// serveBadRequest handles 400 errors and renders err to the bad-request template.
func (s *Server) serveBadRequest(w http.ResponseWriter, req *http.Request, errMsg error) {
	w.WriteHeader(http.StatusBadRequest)

	if err := s.templates.badRequest.ExecuteTemplate(w, "base", errMsg); err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

// Run starts the server that host admin web app and api endpoint
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(s.server.Shutdown(ctx))
	})
	group.Go(func() error {
		defer cancel()
		return Error.Wrap(s.server.Serve(s.listener))
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (s *Server) Close() error {
	return Error.Wrap(s.server.Close())
}
