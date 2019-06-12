// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"time"
	"reflect"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"github.com/gorilla/schema"
	"storj.io/storj/satellite/marketing"
	"github.com/gorilla/mux"
)

// Time converter is used by decoder to format expiration date field from form input
var (
	Error       = errs.Class("satellite marketing error")
	decoder     = schema.NewDecoder()
	timeConverter = func(value string) reflect.Value {
		v, err := time.Parse("2006-01-02",value)
		if err != nil {
			return reflect.Value{}
		}
		return reflect.ValueOf(v)
	}
)

type Config struct {
	Address   string `help:"server address of the marketing Admin GUI" default:"127.0.0.1:8090"`
	StaticDir string `help:"path to static resources" default:""`
}

// Server represents marketing offersweb server
type Server struct {
	log *zap.Logger
	config Config
	listener net.Listener
	server   http.Server
	service  *marketing.Service
	templateDir string
}

// Struct used to render each offer table
type offerSet struct {
	RefOffers,FreeCredits []marketing.Offer
}

// Takes db result of get offers, organizes them by type and returns them
// Used by the index handler to build render the tables
func organizeOffers(offers []marketing.Offer) offerSet{
	os := new(offerSet)
	for _,offer := range offers {
		if offer.Type == marketing.FreeCredit {
			os.FreeCredits = append(os.FreeCredits,offer)
		}else if offer.Type == marketing.Referral {
			os.RefOffers = append(os.RefOffers,offer)
		}
	}
	return *os
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
func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) *Server {
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
		mux.HandleFunc("/", s.getOffers)
		mux.PathPrefix("/static/").Handler(fs)
		mux.HandleFunc("/createFreeCredit", s.createOffer)
		mux.HandleFunc("/createRefOffer", s.createOffer)
	}
	s.server.Handler = mux

	s.templateDir = filepath.Join(s.config.StaticDir, "pages")

	return s
}

// Serves index page and renders offer and credits tables
func (s *Server) getOffers(w http.ResponseWriter, req *http.Request){
	if req.URL.Path != "/" {
		s.serveNotFound(w, req)
		return
	}

	offers, err := s.service.ListAllOffers(context.Background())
	if err != nil {
		s.log.Error("app handler error", zap.Error(err))
		s.serveInternalError(w,req,err)
		return
	}

	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "home.html"),
		filepath.Join(s.templateDir, "refOffers.html"),
		filepath.Join(s.templateDir, "freeOffers.html"),
		filepath.Join(s.templateDir, "roModal.html"),
		filepath.Join(s.templateDir, "foModal.html"),
	)
	home := template.Must(template.New("landingPage").ParseFiles(files...))
	err = home.ExecuteTemplate(w, "base", organizeOffers(offers))
	if err != nil {
		s.serveInternalError(w,req,err)
		return
	}
}

// Helper function used by createOffer to turn post form input into a new offer
func (s *Server) formToStruct(w http.ResponseWriter, req *http.Request) (o marketing.NewOffer, e error){
	err := req.ParseForm()
	if err != nil {
		return o, err
	}
	defer req.Body.Close()

	decoder.RegisterConverter(time.Time{}, timeConverter)

	if err := decoder.Decode(&o, req.PostForm);err != nil {
		return o, err
	}
	return o,nil
}

func (s *Server) createOffer(w http.ResponseWriter, req *http.Request) {
	o, err := s.formToStruct(w,req)
	if err != nil{
		s.log.Error("err from createFreeCredit Handler", zap.Error(err))
		s.serveInternalError(w,req,err)
		return
	}

	o.Status = marketing.Active

	if req.URL.Path == "/createRefOffer" {
		o.Type = marketing.Referral
	}else{
		o.Type = marketing.FreeCredit
	}

	if _, err := s.service.InsertNewOffer(context.Background(), &o); err != nil {
		s.log.Error("createdHandler error", zap.Error(err))
		s.serveInternalError(w,req,err)
		return
	}
	req.Method = "GET"
	http.Redirect(w,req,"/",http.StatusFound)
}

func (s *Server) serveNotFound(w http.ResponseWriter, req *http.Request) {
	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "page-not-found.html"),
	)

	unavailable, err := template.New("page-not-found").ParseFiles(files...)
	if err != nil {
		s.serveInternalError(w, req, err)
		return
	}

	w.WriteHeader(http.StatusNotFound)

	err = unavailable.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
		s.serveInternalError(w,req,err)
		return
	}
}

func (s *Server) serveInternalError(w http.ResponseWriter, req *http.Request, err error) {
	files := append(s.commonPages(),
		filepath.Join(s.templateDir, "internal-server-error.html"),
	)

	unavailable, err := template.New("internal-server-error").ParseFiles(files...)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.log.Error("failed to parse internal server error", zap.Error(err))
		return
	}

	w.WriteHeader(http.StatusInternalServerError)
	err = unavailable.ExecuteTemplate(w, "base", err)
	if err != nil {
		s.log.Error("failed to execute template", zap.Error(err))
	}
}

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

func (s *Server) Close() error {
	return Error.Wrap(s.server.Close())
}
