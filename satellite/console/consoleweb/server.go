// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"
	"encoding/json"
	"html/template"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/referrals"
)

const (
	authorization = "Authorization"
	contentType   = "Content-Type"

	authorizationBearer = "Bearer "

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

var (
	// Error is satellite console error type
	Error = errs.Class("satellite console error")

	mon = monkit.Package()
)

// Config contains configuration for console web server
type Config struct {
	Address         string `help:"server address of the graphql api gateway and frontend app" devDefault:"127.0.0.1:8081" releaseDefault:":10100"`
	StaticDir       string `help:"path to static resources" default:""`
	ExternalAddress string `help:"external endpoint of the satellite if hosted" default:""`

	// TODO: remove after Vanguard release
	AuthToken       string `help:"auth token needed for access to registration token creation endpoint" default:""`
	AuthTokenSecret string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`

	PasswordCost int `internal:"true" help:"password hashing cost (0=automatic)" default:"0"`

	ContactInfoURL        string `help:"url link to contacts page" default:"https://forum.storj.io"`
	FrameAncestors        string `help:"allow domains to embed the satellite in a frame, space separated" default:"tardigrade.io"`
	LetUsKnowURL          string `help:"url link to let us know page" default:"https://storjlabs.atlassian.net/servicedesk/customer/portals"`
	SEO                   string `help:"used to communicate with web crawlers and other web robots" default:"User-agent: *\nDisallow: \nDisallow: /cgi-bin/"`
	SatelliteName         string `help:"used to display at web satellite console" default:"Storj"`
	SatelliteOperator     string `help:"name of organization which set up satellite" default:"Storj Labs" `
	TermsAndConditionsURL string `help:"url link to terms and conditions page" default:"https://storj.io/storage-sla/"`
}

// Server represents console web server
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config           Config
	service          *console.Service
	mailService      *mailservice.Service
	referralsService *referrals.Service

	listener net.Listener
	server   http.Server

	stripePublicKey string

	schema    graphql.Schema
	templates struct {
		index               *template.Template
		notFound            *template.Template
		internalServerError *template.Template
		usageReport         *template.Template
		resetPassword       *template.Template
		success             *template.Template
		activated           *template.Template
	}
}

// NewServer creates new instance of console server.
func NewServer(logger *zap.Logger, config Config, service *console.Service, mailService *mailservice.Service, referralsService *referrals.Service, listener net.Listener, stripePublicKey string) *Server {
	server := Server{
		log:              logger,
		config:           config,
		listener:         listener,
		service:          service,
		mailService:      mailService,
		referralsService: referralsService,
		stripePublicKey:  stripePublicKey,
	}

	logger.Sugar().Debugf("Starting Satellite UI on %s...", server.listener.Addr().String())

	if server.config.ExternalAddress != "" {
		if !strings.HasSuffix(server.config.ExternalAddress, "/") {
			server.config.ExternalAddress += "/"
		}
	} else {
		server.config.ExternalAddress = "http://" + server.listener.Addr().String() + "/"
	}

	router := mux.NewRouter()
	fs := http.FileServer(http.Dir(server.config.StaticDir))

	router.HandleFunc("/api/v0/graphql", server.grapqlHandler)
	router.HandleFunc("/registrationToken/", server.createRegistrationTokenHandler)
	router.HandleFunc("/robots.txt", server.seoHandler)

	referralsController := consoleapi.NewReferrals(logger, referralsService, service, mailService, server.config.ExternalAddress)
	referralsRouter := router.PathPrefix("/api/v0/referrals").Subrouter()
	referralsRouter.Handle("/tokens", server.withAuth(http.HandlerFunc(referralsController.GetTokens))).Methods(http.MethodGet)
	referralsRouter.HandleFunc("/register", referralsController.Register).Methods(http.MethodPost)

	authController := consoleapi.NewAuth(logger, service, mailService, server.config.ExternalAddress, config.LetUsKnowURL, config.TermsAndConditionsURL, config.ContactInfoURL)
	authRouter := router.PathPrefix("/api/v0/auth").Subrouter()
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.GetAccount))).Methods(http.MethodGet)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.UpdateAccount))).Methods(http.MethodPatch)
	authRouter.Handle("/account/change-password", server.withAuth(http.HandlerFunc(authController.ChangePassword))).Methods(http.MethodPost)
	authRouter.Handle("/account/delete", server.withAuth(http.HandlerFunc(authController.DeleteAccount))).Methods(http.MethodPost)
	authRouter.HandleFunc("/token", authController.Token).Methods(http.MethodPost)
	authRouter.HandleFunc("/register", authController.Register).Methods(http.MethodPost)
	authRouter.HandleFunc("/forgot-password/{email}", authController.ForgotPassword).Methods(http.MethodPost)
	authRouter.HandleFunc("/resend-email/{id}", authController.ResendEmail).Methods(http.MethodPost)

	paymentController := consoleapi.NewPayments(logger, service)
	paymentsRouter := router.PathPrefix("/api/v0/payments").Subrouter()
	paymentsRouter.Use(server.withAuth)
	paymentsRouter.HandleFunc("/cards", paymentController.AddCreditCard).Methods(http.MethodPost)
	paymentsRouter.HandleFunc("/cards", paymentController.MakeCreditCardDefault).Methods(http.MethodPatch)
	paymentsRouter.HandleFunc("/cards", paymentController.ListCreditCards).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/cards/{cardId}", paymentController.RemoveCreditCard).Methods(http.MethodDelete)
	paymentsRouter.HandleFunc("/account/charges", paymentController.ProjectsCharges).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/account/balance", paymentController.AccountBalance).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/account", paymentController.SetupAccount).Methods(http.MethodPost)
	paymentsRouter.HandleFunc("/billing-history", paymentController.BillingHistory).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/tokens/deposit", paymentController.TokenDeposit).Methods(http.MethodPost)

	if server.config.StaticDir != "" {
		router.HandleFunc("/activation/", server.accountActivationHandler)
		router.HandleFunc("/password-recovery/", server.passwordRecoveryHandler)
		router.HandleFunc("/cancel-password-recovery/", server.cancelPasswordRecoveryHandler)
		router.HandleFunc("/usage-report", server.bucketUsageReportHandler)
		router.PathPrefix("/static/").Handler(server.gzipMiddleware(http.StripPrefix("/static", fs)))
		router.PathPrefix("/").Handler(http.HandlerFunc(server.appHandler))
	}

	server.server = http.Server{
		Handler:        router,
		MaxHeaderBytes: ContentLengthLimit.Int(),
	}

	return &server
}

// Run starts the server that host webapp and api endpoint
func (server *Server) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	server.schema, err = consoleql.CreateSchema(server.log, server.service, server.mailService)
	if err != nil {
		return Error.Wrap(err)
	}

	err = server.initializeTemplates()
	if err != nil {
		// TODO: should it return error if some template can not be initialized or just log about it?
		return Error.Wrap(err)
	}

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return server.server.Shutdown(context.Background())
	})
	group.Go(func() error {
		defer cancel()
		return server.server.Serve(server.listener)
	})

	return group.Wait()
}

// Close closes server and underlying listener
func (server *Server) Close() error {
	return server.server.Close()
}

// appHandler is web app http handler function
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	cspValues := []string{
		"default-src 'self'",
		"frame-ancestors " + server.config.FrameAncestors,
		"frame-src 'self' *.stripe.com",
		"img-src 'self' data:",
		"script-src 'self' *.stripe.com cdn.segment.com",
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin") // Only expose the referring url when navigating around the satellite itself.

	var data struct {
		SatelliteName   string
		StripePublicKey string
	}

	data.SatelliteName = server.config.SatelliteName
	data.StripePublicKey = server.stripePublicKey

	if server.templates.index == nil || server.templates.index.Execute(w, data) != nil {
		server.log.Error("index template could not be executed")
		return
	}
}

// authMiddlewareHandler performs initial authorization before every request.
func (server *Server) withAuth(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var err error
		defer mon.Task()(&ctx)(&err)
		token := getToken(r)

		ctx = auth.WithAPIKey(ctx, []byte(token))
		auth, err := server.service.Authorize(ctx)
		if err != nil {
			ctx = console.WithAuthFailure(ctx, err)
		} else {
			ctx = console.WithAuth(ctx, auth)
		}

		handler.ServeHTTP(w, r.Clone(ctx))
	})
}

// bucketUsageReportHandler generate bucket usage report page for project
func (server *Server) bucketUsageReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	tokenCookie, err := r.Cookie("_tokenKey")
	if err != nil {
		server.serveError(w, http.StatusUnauthorized)
		return
	}

	auth, err := server.service.Authorize(auth.WithAPIKey(ctx, []byte(tokenCookie.Value)))
	if err != nil {
		server.serveError(w, http.StatusUnauthorized)
		return
	}

	ctx = console.WithAuth(ctx, auth)

	// parse query params
	projectID, err := uuid.Parse(r.URL.Query().Get("projectID"))
	if err != nil {
		server.serveError(w, http.StatusBadRequest)
		return
	}
	sinceStamp, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if err != nil {
		server.serveError(w, http.StatusBadRequest)
		return
	}
	beforeStamp, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if err != nil {
		server.serveError(w, http.StatusBadRequest)
		return
	}

	since := time.Unix(sinceStamp, 0)
	before := time.Unix(beforeStamp, 0)

	server.log.Debug("querying bucket usage report",
		zap.Stringer("projectID", projectID),
		zap.Stringer("since", since),
		zap.Stringer("before", before))

	bucketRollups, err := server.service.GetBucketUsageRollups(ctx, *projectID, since, before)
	if err != nil {
		server.log.Error("bucket usage report error", zap.Error(err))
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	if err = server.templates.usageReport.Execute(w, bucketRollups); err != nil {
		server.log.Error("bucket usage report error", zap.Error(err))
	}
}

// accountActivationHandler is web app http handler function
func (server *Server) createRegistrationTokenHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	w.Header().Set(contentType, applicationJSON)

	var response struct {
		Secret string `json:"secret"`
		Error  string `json:"error,omitempty"`
	}

	defer func() {
		err := json.NewEncoder(w).Encode(&response)
		if err != nil {
			server.log.Error(err.Error())
		}
	}()

	authToken := r.Header.Get("Authorization")
	if authToken != server.config.AuthToken {
		w.WriteHeader(401)
		response.Error = "unauthorized"
		return
	}

	projectsLimitInput := r.URL.Query().Get("projectsLimit")

	projectsLimit, err := strconv.Atoi(projectsLimitInput)
	if err != nil {
		response.Error = err.Error()
		return
	}

	token, err := server.service.CreateRegToken(ctx, projectsLimit)
	if err != nil {
		response.Error = err.Error()
		return
	}

	response.Secret = token.Secret.String()
}

// accountActivationHandler is web app http handler function
func (server *Server) accountActivationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	activationToken := r.URL.Query().Get("token")

	err := server.service.ActivateAccount(ctx, activationToken)
	if err != nil {
		server.log.Error("activation: failed to activate account",
			zap.String("token", activationToken),
			zap.Error(err))

		// TODO: when new error pages will be created - change http.StatusNotFound on appropriate one
		server.serveError(w, http.StatusNotFound)
		return
	}

	if err = server.templates.activated.Execute(w, nil); err != nil {
		server.log.Error("account activated template could not be executed", zap.Error(Error.Wrap(err)))
		return
	}
}

func (server *Server) passwordRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	recoveryToken := r.URL.Query().Get("token")
	if len(recoveryToken) == 0 {
		server.serveError(w, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			server.serveError(w, http.StatusNotFound)
			return
		}

		password := r.FormValue("password")
		passwordRepeat := r.FormValue("passwordRepeat")
		if strings.Compare(password, passwordRepeat) != 0 {
			server.serveError(w, http.StatusNotFound)
			return
		}

		err = server.service.ResetPassword(ctx, recoveryToken, password)
		if err != nil {
			server.serveError(w, http.StatusNotFound)
			return
		}

		if err := server.templates.success.Execute(w, nil); err != nil {
			server.log.Error("success reset password template could not be executed", zap.Error(Error.Wrap(err)))
			return
		}
	case http.MethodGet:
		if err := server.templates.resetPassword.Execute(w, nil); err != nil {
			server.log.Error("reset password template could not be executed", zap.Error(Error.Wrap(err)))
			return
		}
	default:
		server.serveError(w, http.StatusNotFound)
		return
	}
}

func (server *Server) cancelPasswordRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	recoveryToken := r.URL.Query().Get("token")

	// No need to check error as we anyway redirect user to support page
	_ = server.service.RevokeResetPasswordToken(ctx, recoveryToken)

	// TODO: Should place this link to config
	http.Redirect(w, r, "https://storjlabs.atlassian.net/servicedesk/customer/portals", http.StatusSeeOther)
}

func (server *Server) serveError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)

	switch status {
	case http.StatusInternalServerError:
		err := server.templates.internalServerError.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse internalServerError template", zap.Error(Error.Wrap(err)))
		}
	default:
		err := server.templates.notFound.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse pageNotFound template", zap.Error(Error.Wrap(err)))
		}
	}
}

// grapqlHandler is graphql endpoint http handler function
func (server *Server) grapqlHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	w.Header().Set(contentType, applicationJSON)

	token := getToken(r)
	query, err := getQuery(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx = auth.WithAPIKey(ctx, []byte(token))
	auth, err := server.service.Authorize(ctx)
	if err != nil {
		ctx = console.WithAuthFailure(ctx, err)
	} else {
		ctx = console.WithAuth(ctx, auth)
	}

	rootObject := make(map[string]interface{})

	rootObject["origin"] = server.config.ExternalAddress
	rootObject[consoleql.ActivationPath] = "activation/?token="
	rootObject[consoleql.PasswordRecoveryPath] = "password-recovery/?token="
	rootObject[consoleql.CancelPasswordRecoveryPath] = "cancel-password-recovery/?token="
	rootObject[consoleql.SignInPath] = "login"
	rootObject[consoleql.LetUsKnowURL] = server.config.LetUsKnowURL
	rootObject[consoleql.ContactInfoURL] = server.config.ContactInfoURL
	rootObject[consoleql.TermsAndConditionsURL] = server.config.TermsAndConditionsURL

	result := graphql.Do(graphql.Params{
		Schema:         server.schema,
		Context:        ctx,
		RequestString:  query.Query,
		VariableValues: query.Variables,
		OperationName:  query.OperationName,
		RootObject:     rootObject,
	})

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		server.log.Error(err.Error())
		return
	}

	sugar := server.log.Sugar()
	sugar.Debug(result)
}

// seoHandler used to communicate with web crawlers and other web robots
func (server *Server) seoHandler(w http.ResponseWriter, req *http.Request) {
	header := w.Header()

	header.Set(contentType, mime.TypeByExtension(".txt"))
	header.Set("X-Content-Type-Options", "nosniff")

	_, err := w.Write([]byte(server.config.SEO))
	if err != nil {
		server.log.Error(err.Error())
	}
}

// gzipMiddleware is used to gzip static content to minify resources if browser support such decoding.
func (server *Server) gzipMiddleware(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		isGzipSupported := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
		if !isGzipSupported {
			fn.ServeHTTP(w, r)
			return
		}

		info, err := os.Stat(server.config.StaticDir + strings.TrimPrefix(r.URL.Path, "/static") + ".gz")
		if err != nil {
			fn.ServeHTTP(w, r)
			return
		}

		extension := filepath.Ext(info.Name()[:len(info.Name())-3])
		w.Header().Set(contentType, mime.TypeByExtension(extension))
		w.Header().Set("Content-Encoding", "gzip")

		newRequest := new(http.Request)
		*newRequest = *r
		newRequest.URL = new(url.URL)
		*newRequest.URL = *r.URL
		newRequest.URL.Path += ".gz"

		fn.ServeHTTP(w, newRequest)
	})
}

// initializeTemplates is used to initialize all templates
func (server *Server) initializeTemplates() (err error) {
	server.templates.index, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "dist", "index.html"))
	if err != nil {
		server.log.Error("dist folder is not generated. use 'npm run build' command", zap.Error(err))
	}

	server.templates.activated, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "activation", "activated.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	server.templates.success, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "resetPassword", "success.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	server.templates.resetPassword, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "resetPassword", "resetPassword.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	server.templates.usageReport, err = template.ParseFiles(path.Join(server.config.StaticDir, "static", "reports", "usageReport.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	server.templates.notFound, err = template.ParseFiles(path.Join(server.config.StaticDir, "static", "errors", "404.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	server.templates.internalServerError, err = template.ParseFiles(path.Join(server.config.StaticDir, "static", "errors", "500.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
