// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/satellite/marketing"
)

var (
	// Error is satellite marketing error type
	Error   = errs.Class("satellite marketing error")
	decoder = schema.NewDecoder()
)

// Config contains configuration for marketingweb server
type Config struct {
	Address   string `help:"server address of the marketing Admin GUI" default:"127.0.0.1:8090"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents marketing offersweb server
type Server struct {
	log         *zap.Logger
	Config      Config
	listener    net.Listener
	server      http.Server
	service     *marketing.Service
	templateDir string
	templates   struct {
		home          *template.Template
		pageNotFound  *template.Template
		internalError *template.Template
	}
}

// offerSet provides a separation of marketing offers by type.
type offerSet struct {
	RefOffers, FreeCredits []marketing.Offer
}

// init safely registers the timeConverter for the decoder.
func init() {
	decoder.RegisterConverter(time.Time{}, timeConverter)
}

// organizeOffers organizes offers by type.
func organizeOffers(offers []marketing.Offer) offerSet {
	var os offerSet
	for _, offer := range offers {

		switch offer.Type {

		case marketing.FreeCredit:
			os.FreeCredits = append(os.FreeCredits, offer)

		case marketing.Referral:
			os.RefOffers = append(os.RefOffers, offer)
		}

	}
	return os
}

// CommonPages returns templates that are required for everything.
func (s *Server) commonPages() []string {
	return []string{
		filepath.Join(s.templateDir, "base.html"),
		filepath.Join(s.templateDir, "index.html"),
		filepath.Join(s.templateDir, "banner.html"),
		filepath.Join(s.templateDir, "logo.html"),
	}
}

// NewServer creates new instance of offersweb server
func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) (*Server, error) {
	s := &Server{
		log:      logger,
		Config:   config,
		listener: listener,
		service:  service,
	}

	decoder.RegisterConverter(time.Time{}, timeConverter)

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", s.listener.Addr().String())
	fs := http.StripPrefix("/static/", http.FileServer(http.Dir(s.Config.StaticDir)))
	mux := mux.NewRouter()
	if s.Config.StaticDir != "" {
		mux.HandleFunc("/", s.getOffers)
		mux.PathPrefix("/static/").Handler(fs)
		mux.HandleFunc("/create/{offer_type}", s.CreateOffer)
	}
	s.server.Handler = mux

	s.templateDir = filepath.Join(s.Config.StaticDir, "pages")

	if err := s.parseTemplates(); err != nil {
		return nil, Error.Wrap(err)
	}

	return s, nil
}

// getOffers renders the tables for free credits and referral offers to the UI
func (s *Server) getOffers(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/" {
		s.serveNotFound(w, req)
		return
	}

	offers, err := s.service.ListAllOffers(context.Background())
	if err != nil {
		s.log.Error("failed to retrieve all offers", zap.Error(err))
		s.serveInternalError(w, req, err)
		return
	}

	if err := s.templates.home.ExecuteTemplate(w, "base", organizeOffers(offers)); err != nil {
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
	)

	pageNotFoundFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "page-not-found.html"),
	)

	internalErrorFiles := append(s.commonPages(),
		filepath.Join(s.templateDir, "internal-server-error.html"),
	)

	s.templates.home, err = template.New("landingPage").ParseFiles(homeFiles...)
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

	return nil
}

// timeConverter formats form time input as time.Time.
func timeConverter(value string) reflect.Value {
	v, err := time.Parse("2006-01-02", value)
	if err != nil {
		return reflect.Value{}
	}
	return reflect.ValueOf(v)
}

// formToStruct decodes POST form data into a new offer.
func formToStruct(w http.ResponseWriter, req *http.Request) (o marketing.NewOffer, e error) {
	err := req.ParseForm()
	if err != nil {
		return o, err
	}

	if err := decoder.Decode(&o, req.PostForm); err != nil {
		return o, err
	}
	return o, nil
}

// CreateOffer handles requests to create new offers.
func (s *Server) CreateOffer(w http.ResponseWriter, req *http.Request) {
	o, err := formToStruct(w, req)
	if err != nil {
		s.log.Error("failed to convert form to struct", zap.Error(err))
		s.serveInternalError(w, req, err)
		return
	}

	o.Status = marketing.Active
	reqType := mux.Vars(req)["offer_type"]

	if reqType == "referral-offer" {
		o.Type = marketing.Referral
	} else {
		o.Type = marketing.FreeCredit
	}

	if _, err := s.service.InsertNewOffer(context.Background(), &o); err != nil {
		s.log.Error("failed to insert new offer", zap.Error(err))
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
		s.serveInternalError(w, req, err)
		return
	}
}

// serveInternalError handles 500 errors and renders err to the internalErr template.
func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request, e error) {

	w.WriteHeader(http.StatusInternalServerError)

	if err := s.templates.internalError.ExecuteTemplate(w, "base", e); err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

// Run starts the server that host admin web app and api endpoint
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return Error.Wrap(s.server.Shutdown(nil))
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
