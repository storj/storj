// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package marketingweb

import (
	"context"
	"html/template"
	"net"
	"net/http"
	"fmt"
	"time"
	"reflect"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"github.com/gorilla/schema"
	"storj.io/storj/satellite/marketing"
)

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
	Address		string `help:"server address of the marketing Admin GUI" default:"0.0.0.0:8090"`
	StaticDir	string `help:"path to static resources" default:""`
}

// Server represents marketing offersweb server
type Server struct {
	log *zap.Logger
	config Config
	listener net.Listener
	server   http.Server
	service  *marketing.Service
}

type offerSet struct {
	RefOffers,FreeCredits				[]marketing.Offer
}

// The three pages contained in addPages are pages all templates require
// This exists in order to limit handler verbosity
func (s *Server) addPages(assets []string) []string {
	rp :=  s.config.StaticDir + "/pages/"
	pages := []string{rp + "base.html", rp + "index.html", rp + "banner.html", rp + "logo.html"}
	for _, page := range assets {
		pages = append(pages, page)
	}
	return pages
}

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

func NewServer(logger *zap.Logger, config Config, service *marketing.Service, listener net.Listener) *Server {
	server := Server{
		log:      logger,
		config:   config,
		listener: listener,
		service:  service,
	}

	logger.Sugar().Debugf("Starting Marketing Admin UI on %s...", server.listener.Addr().String())
	fs := http.FileServer(http.Dir(server.config.StaticDir))
	s := http.StripPrefix("/static/", fs)
	mux := mux.NewRouter()

	if server.config.StaticDir != "" {
		mux.HandleFunc("/", server.getOffers)
		mux.PathPrefix("/static/").Handler(s)
		mux.HandleFunc("/createFreeCredit", server.createOffer)
		mux.HandleFunc("/createRefOffer", server.createOffer)
	}
	server.server = http.Server{
		Handler: mux,
	}

	return &server
}

func (s *Server) getOffers(w http.ResponseWriter, req *http.Request){
	offers, err := s.service.ListAllOffers(context.Background())
	if err != nil {
		s.log.Error("app handler error", zap.Error(err))
		s.serveError(w, req)
		return
	}
	rp := s.config.StaticDir + "/pages/"
	pages := []string{rp + "home.html", rp + "refOffers.html", rp + "freeOffers.html", rp + "roModal.html", rp + "foModal.html"}
	files := s.addPages(pages)
	home := template.Must(template.New("landingPage").ParseFiles(files...))
	err = home.ExecuteTemplate(w, "base", organizeOffers(offers))
	if err != nil {
		fmt.Println(err)
		s.serveError(w, req)
	}
}

func (s *Server) formToStruct(w http.ResponseWriter, req *http.Request) (o marketing.NewOffer, e error){
	err := req.ParseForm()
	if err != nil {
		fmt.Printf("err parsing form : %v\n", err)
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
		s.serveError(w, req)
	}

	o.Status = marketing.Active

	if req.URL.Path == "/createRefOffer" {
		o.Type = marketing.Referral
	}else{
		o.Type = marketing.FreeCredit
	}

	if _, err := s.service.InsertNewOffer(context.Background(), &o); err != nil {
		s.log.Error("createdHandler error", zap.Error(err))
		rp := s.config.StaticDir + "/pages/"
		files := s.addPages([]string{rp + "err.html"})
		errPage := template.Must(template.New("err").ParseFiles(files...))
		errPage.ExecuteTemplate(w, "base", err)
		return
	}
	req.Method = "GET"
	http.Redirect(w,req,"/",http.StatusFound)
}

func (s *Server) serveError(w http.ResponseWriter, req *http.Request) {
	rp := s.config.StaticDir + "/pages/"
	files := s.addPages([]string{rp + "404.html"})
	unavailable := template.Must(template.New("404").ParseFiles(files...))
	err := unavailable.ExecuteTemplate(w, "base", nil)
	if err != nil {
		s.serveError(w, req)
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
