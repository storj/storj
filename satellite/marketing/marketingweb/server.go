// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"strconv"
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
	config      Config
	listener    net.Listener
	server      http.Server
	service     *marketing.Service
	templateDir string
	templates   struct {
		home          *template.Template
		pageNotFound  *template.Template
		internalError *template.Template
		badRequest    *template.Template
	}
}

// offerSet provides a separation of marketing offers by type.
type offerSet struct {
	ReferralOffers, FreeCredits []marketing.Offer
}

// init safely registers convertStringToTime for the decoder.
func init() {
	decoder.RegisterConverter(time.Time{}, convertStringToTime)
}

// organizeOffers organizes offers by type.
func organizeOffers(offers []marketing.Offer) offerSet {
	var os offerSet
	for _, offer := range offers {

		switch offer.Type {
		case marketing.FreeCredit:
			os.FreeCredits = append(os.FreeCredits, offer)
		case marketing.Referral:
			os.ReferralOffers = append(os.ReferralOffers, offer)
		default:
			continue
		}

	}
	return os
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
func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) (*Server, error) {
	s := &Server{
		log:      logger,
		config:   config,
		listener: listener,
		service:  service,
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

	offers, err := s.service.ListAllOffers(req.Context())
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
		filepath.Join(s.templateDir, "stop-referral-offer.html"),
		filepath.Join(s.templateDir, "stop-free-credit.html"),
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

	s.templates.badRequest, err = template.New("bad-request-error").ParseFiles(badRequestFiles...)
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}

// convertStringToTime formats form time input as time.Time.
func convertStringToTime(value string) reflect.Value {
	v, err := time.Parse("2006-01-02", value)
	if err != nil {
		log.Println("invalid decoder value")
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
		s.serveBadRequest(w, req, err)
		return
	}

	o.Status = marketing.Active
	reqType := mux.Vars(req)["offer_type"]

	switch reqType {
	case "referral-offer":
		o.Type = marketing.Referral
	case "free-credit":
		o.Type = marketing.FreeCredit
	default:
		err := errs.New("response status %d : invalid offer type", http.StatusBadRequest)
		s.serveBadRequest(w, req, err)
		return
	}

	if _, err := s.service.InsertNewOffer(req.Context(), &o); err != nil {
		s.log.Error("failed to insert new offer", zap.Error(err))
		s.serveBadRequest(w, req, err)
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

// StopOffer expires the current offer and replaces it with the default offer.
func (s *Server) StopOffer(w http.ResponseWriter, req *http.Request) {
	offerID, err := strconv.Atoi(mux.Vars(req)["offer_id"])
	if err != nil {
		s.log.Error("failed to parse offer id", zap.Error(err))
		s.serveBadRequest(w, req, err)
		return
	}

	if err := s.service.FinishOffer(context.Background(), offerID); err != nil {
		s.log.Error("failed to stop offer", zap.Error(err))
		s.serveInternalError(w, req, err)
		return
	}

	http.Redirect(w, req, "/", http.StatusSeeOther)
}

// Handler for 500 errors and also renders err to the internalErr template.
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
