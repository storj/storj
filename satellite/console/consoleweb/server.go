// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/spkg/zipfs"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
)

const (
	authorization = "Authorization"
	contentType   = "Content-Type"

	authorizationBearer = "Bearer "

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

// Error is satellite console error type
var Error = errs.Class("satellite console error")

const landingPage = "dist/public/index.html"

// Config contains configuration for console web server
type Config struct {
	Address         string `help:"server address of the graphql api gateway and frontend app" default:"127.0.0.1:8081"`
	StaticArchive   string `help:"path to static resources zip" default:"assets.zip"`
	StaticDir       string `help:"path to static resources folder" default:""`
	ExternalAddress string `help:"external endpoint of the satellite if hosted" default:""`

	// TODO: remove after Vanguard release
	AuthToken string `help:"auth token needed for access to registration token creation endpoint" default:""`

	PasswordCost int `internal:"true" help:"password hashing cost (0=automatic)" default:"0"`
}

// Server represents console web server
type Server struct {
	log *zap.Logger

	config      Config
	service     *console.Service
	mailService *mailservice.Service

	listener net.Listener
	server   http.Server
	assets   http.FileSystem

	schema graphql.Schema
}

// OpenServer creates new instance of console server
func OpenServer(logger *zap.Logger, config Config, service *console.Service, mailService *mailservice.Service, listener net.Listener) (*Server, error) {
	server := Server{
		log:         logger,
		config:      config,
		listener:    listener,
		service:     service,
		mailService: mailService,
	}

	logger.Debug("Starting Satellite UI...")

	if server.config.ExternalAddress != "" {
		if !strings.HasSuffix(server.config.ExternalAddress, "/") {
			server.config.ExternalAddress = server.config.ExternalAddress + "/"
		}
	} else {
		server.config.ExternalAddress = "http://" + server.listener.Addr().String() + "/"
	}

	_, err := os.Stat(filepath.Join(config.StaticDir, landingPage))
	switch {
	case err == nil:
		server.assets = http.Dir(config.StaticDir)
	case os.IsNotExist(err):
		zipfile, err := zipfs.New(server.config.StaticArchive)
		if err != nil {
			return nil, Error.New("failed to find static assets in either static-dir or static-archive: %v", err)
		}
		server.assets = zipfile
	case err != nil:
		return nil, Error.New("failed looking for static assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/graphql/v0", http.HandlerFunc(server.grapqlHandler))
	if server.config.StaticArchive != "" {
		mux.Handle("/activation/", http.HandlerFunc(server.accountActivationHandler))
		mux.Handle("/password-recovery/", http.HandlerFunc(server.passwordRecoveryHandler))
		mux.Handle("/registrationToken/", http.HandlerFunc(server.createRegistrationTokenHandler))
		mux.Handle("/usage-report/", http.HandlerFunc(server.bucketUsageReportHandler))
		mux.Handle("/static/", http.StripPrefix("/static", http.FileServer(server.assets)))
		mux.Handle("/", http.HandlerFunc(server.appHandler))
	}

	server.server = http.Server{
		Handler: mux,
	}

	return &server, nil
}

// serveFile serves a single file from the assets archive to the requester
func (s *Server) serveFile(w http.ResponseWriter, req *http.Request, path ...string) {
	f, err := s.assets.Open(filepath.Join(path...))
	if err != nil {
		if os.IsNotExist(err) {
			s.serveErr(w, req, http.StatusNotFound, err)
			return
		}
		s.serveErr(w, req, http.StatusInternalServerError, err)
		return
	}
	stat, err := f.Stat()
	if err != nil {
		s.serveErr(w, req, http.StatusInternalServerError, err)
		return
	}
	http.ServeContent(w, req, path[len(path)-1], stat.ModTime(), f)
}

// appHandler is web app http handler function
func (s *Server) appHandler(w http.ResponseWriter, req *http.Request) {
	s.serveFile(w, req, landingPage)
}

// bucketUsageReportHandler generate bucket usage report page for project
func (s *Server) bucketUsageReportHandler(w http.ResponseWriter, req *http.Request) {
	var err error

	var projectID *uuid.UUID
	var since, before time.Time

	tokenCookie, err := req.Cookie("tokenKey")
	if err != nil {
		s.log.Error("bucket usage report error", zap.Error(err))
		s.serveErr(w, req, http.StatusUnauthorized, err)
		return
	}

	auth, err := s.service.Authorize(auth.WithAPIKey(req.Context(), []byte(tokenCookie.Value)))
	if err != nil {
		s.serveErr(w, req, http.StatusUnauthorized, err)
		return
	}

	defer func() {
		if err != nil {
			s.serveErr(w, req, http.StatusNotFound, err)
		}
	}()

	// parse query params
	projectID, err = uuid.Parse(req.URL.Query().Get("projectID"))
	if err != nil {
		return
	}
	since, err = time.Parse(time.RFC3339, req.URL.Query().Get("since"))
	if err != nil {
		return
	}
	before, err = time.Parse(time.RFC3339, req.URL.Query().Get("before"))
	if err != nil {
		return
	}

	s.log.Debug("querying bucket usage report",
		zap.String("projectID", projectID.String()),
		zap.String("since", since.String()),
		zap.String("before", before.String()))

	ctx := console.WithAuth(context.Background(), auth)
	bucketRollups, err := s.service.GetBucketUsageRollups(ctx, *projectID, since, before)
	if err != nil {
		return
	}

	report, err := s.getParsedTemplate("static/reports/UsageReport.html")
	if err != nil {
		return
	}

	err = report.Execute(w, bucketRollups)
}

// accountActivationHandler is web app http handler function
func (s *Server) createRegistrationTokenHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(contentType, applicationJSON)

	var response struct {
		Secret string `json:"secret"`
		Error  string `json:"error,omitempty"`
	}

	defer func() {
		err := json.NewEncoder(w).Encode(&response)
		if err != nil {
			s.log.Error(err.Error())
		}
	}()

	authToken := req.Header.Get("Authorization")
	if authToken != s.config.AuthToken {
		w.WriteHeader(401)
		response.Error = "unauthorized"
		return
	}

	projectsLimitInput := req.URL.Query().Get("projectsLimit")

	projectsLimit, err := strconv.Atoi(projectsLimitInput)
	if err != nil {
		response.Error = err.Error()
		return
	}

	token, err := s.service.CreateRegToken(context.Background(), projectsLimit)
	if err != nil {
		response.Error = err.Error()
		return
	}

	response.Secret = token.Secret.String()
}

// accountActivationHandler is web app http handler function
func (s *Server) accountActivationHandler(w http.ResponseWriter, req *http.Request) {
	activationToken := req.URL.Query().Get("token")

	err := s.service.ActivateAccount(context.Background(), activationToken)
	if err != nil {
		s.log.Error("activation: failed to activate account",
			zap.String("token", activationToken),
			zap.Error(err))
		s.serveErr(w, req, http.StatusUnauthorized, err)
		return
	}

	s.serveFile(w, req, "static", "activation", "success.html")
}

func (s *Server) passwordRecoveryHandler(w http.ResponseWriter, req *http.Request) {
	recoveryToken := req.URL.Query().Get("token")
	if len(recoveryToken) == 0 {
		s.serveErr(w, req, http.StatusUnauthorized, nil)
		return
	}

	switch req.Method {
	case "POST":
		err := req.ParseForm()
		if err != nil {
			s.serveErr(w, req, http.StatusBadRequest, err)
			return
		}

		password := req.FormValue("password")
		passwordRepeat := req.FormValue("passwordRepeat")
		if strings.Compare(password, passwordRepeat) != 0 {
			s.serveErr(w, req, http.StatusBadRequest, err)
			return
		}

		err = s.service.ResetPassword(context.Background(), recoveryToken, password)
		if err != nil {
			s.serveErr(w, req, http.StatusBadRequest, err)
			return
		}
	default:
		t, err := s.getParsedTemplate("static/reports/resetPassword.html")
		if err != nil {
			s.serveErr(w, req, http.StatusInternalServerError, err)
			return
		}

		err = t.Execute(w, nil)
		if err != nil {
			s.serveErr(w, req, http.StatusBadRequest, err)
			return
		}
	}
}

func (s *Server) getParsedTemplate(path string) (*template.Template, error) {
	// TODO: cache these?
	content, err := s.getFromArchive(path)
	if err != nil {
		return nil, err
	}
	return template.New("tmpl").Parse(string(content))
}

func (s *Server) getFromArchive(path string) ([]byte, error) {
	f, err := s.assets.Open(filepath.Join(path))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	contents, err := ioutil.ReadAll(f)
	return contents, Error.Wrap(err)

}

func (s *Server) serveErr(w http.ResponseWriter, req *http.Request, statusCode int, err error) {
	if statusCode != http.StatusNotFound {
		s.log.Error("error", zap.Error(err))
	}
	w.WriteHeader(statusCode)
	// TODO: choose a template based on status code
	s.serveFile(w, req, "static", "errors", "404.html")
}

// grapqlHandler is graphql endpoint http handler function
func (s *Server) grapqlHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set(contentType, applicationJSON)

	token := getToken(req)
	query, err := getQuery(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := auth.WithAPIKey(context.Background(), []byte(token))
	auth, err := s.service.Authorize(ctx)
	if err != nil {
		ctx = console.WithAuthFailure(ctx, err)
	} else {
		ctx = console.WithAuth(ctx, auth)
	}

	rootObject := make(map[string]interface{})

	rootObject["origin"] = s.config.ExternalAddress
	rootObject[consoleql.ActivationPath] = "activation/?token="
	rootObject[consoleql.PasswordRecoveryPath] = "password-recovery/?token="
	rootObject[consoleql.SignInPath] = "login"

	result := graphql.Do(graphql.Params{
		Schema:         s.schema,
		Context:        ctx,
		RequestString:  query.Query,
		VariableValues: query.Variables,
		OperationName:  query.OperationName,
		RootObject:     rootObject,
	})

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		s.log.Error(err.Error())
		return
	}

	sugar := s.log.Sugar()
	sugar.Debug(result)
}

// Run starts the server that host webapp and api endpoint
func (s *Server) Run(ctx context.Context) error {
	var err error

	s.schema, err = consoleql.CreateSchema(s.log, s.service, s.mailService)
	if err != nil {
		return Error.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return s.server.Shutdown(nil)
	})
	group.Go(func() error {
		defer cancel()
		return s.server.Serve(s.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (s *Server) Close() error {
	return s.server.Close()
}
