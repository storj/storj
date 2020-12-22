// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
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
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/accounting"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/referrals"
	"storj.io/storj/satellite/rewards"
)

const (
	contentType = "Content-Type"

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

var (
	// Error is satellite console error type.
	Error = errs.Class("satellite console error")

	mon = monkit.Package()
)

// Config contains configuration for console web server.
type Config struct {
	Address         string `help:"server address of the graphql api gateway and frontend app" devDefault:"" releaseDefault:":10100"`
	StaticDir       string `help:"path to static resources" default:""`
	ExternalAddress string `help:"external endpoint of the satellite if hosted" default:""`

	// TODO: remove after Vanguard release
	AuthToken       string `help:"auth token needed for access to registration token creation endpoint" default:""`
	AuthTokenSecret string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`

	ContactInfoURL                  string `help:"url link to contacts page" default:"https://forum.storj.io"`
	FrameAncestors                  string `help:"allow domains to embed the satellite in a frame, space separated" default:"tardigrade.io"`
	LetUsKnowURL                    string `help:"url link to let us know page" default:"https://storjlabs.atlassian.net/servicedesk/customer/portals"`
	SEO                             string `help:"used to communicate with web crawlers and other web robots" default:"User-agent: *\nDisallow: \nDisallow: /cgi-bin/"`
	SatelliteName                   string `help:"used to display at web satellite console" default:"Storj"`
	SatelliteOperator               string `help:"name of organization which set up satellite" default:"Storj Labs" `
	TermsAndConditionsURL           string `help:"url link to terms and conditions page" default:"https://storj.io/storage-sla/"`
	SegmentIOPublicKey              string `help:"used to initialize segment.io at web satellite console" default:""`
	AccountActivationRedirectURL    string `help:"url link for account activation redirect" default:""`
	VerificationPageURL             string `help:"url link to sign up verification page" default:"https://tardigrade.io/verify"`
	PartneredSatelliteNames         string `help:"names of partnered satellites" default:"US-Central-1,Europe-West-1,Asia-East-1"`
	GoogleTagManagerID              string `help:"id for google tag manager" default:""`
	GeneralRequestURL               string `help:"url link to general request page" default:"https://support.tardigrade.io/hc/en-us/requests/new?ticket_form_id=360000379291"`
	ProjectLimitsIncreaseRequestURL string `help:"url link to project limit increase request page" default:"https://support.tardigrade.io/hc/en-us/requests/new?ticket_form_id=360000683212"`
	GatewayCredentialsRequestURL    string `help:"url link for gateway credentials requests" default:"https://auth.tardigradeshare.io"`

	RateLimit web.IPRateLimiterConfig

	console.Config
}

// Server represents console web server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config           Config
	service          *console.Service
	mailService      *mailservice.Service
	referralsService *referrals.Service
	partners         *rewards.PartnersService

	listener    net.Listener
	server      http.Server
	cookieAuth  *consolewebauth.CookieAuth
	rateLimiter *web.IPRateLimiter
	nodeURL     storj.NodeURL

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
func NewServer(logger *zap.Logger, config Config, service *console.Service, mailService *mailservice.Service, referralsService *referrals.Service, partners *rewards.PartnersService, listener net.Listener, stripePublicKey string, nodeURL storj.NodeURL) *Server {
	server := Server{
		log:              logger,
		config:           config,
		listener:         listener,
		service:          service,
		mailService:      mailService,
		referralsService: referralsService,
		partners:         partners,
		stripePublicKey:  stripePublicKey,
		rateLimiter:      web.NewIPRateLimiter(config.RateLimit),
		nodeURL:          nodeURL,
	}

	logger.Debug("Starting Satellite UI.", zap.Stringer("Address", server.listener.Addr()))

	server.cookieAuth = consolewebauth.NewCookieAuth(consolewebauth.CookieSettings{
		Name: "_tokenKey",
		Path: "/",
	})

	if server.config.ExternalAddress != "" {
		if !strings.HasSuffix(server.config.ExternalAddress, "/") {
			server.config.ExternalAddress += "/"
		}
	} else {
		server.config.ExternalAddress = "http://" + server.listener.Addr().String() + "/"
	}

	if server.config.AccountActivationRedirectURL == "" {
		server.config.AccountActivationRedirectURL = server.config.ExternalAddress + "login?activated=true"
	}

	router := mux.NewRouter()
	fs := http.FileServer(http.Dir(server.config.StaticDir))

	router.HandleFunc("/registrationToken/", server.createRegistrationTokenHandler)
	router.HandleFunc("/populate-promotional-coupons", server.populatePromotionalCoupons).Methods(http.MethodPost)
	router.HandleFunc("/robots.txt", server.seoHandler)

	router.Handle("/api/v0/graphql", server.withAuth(http.HandlerFunc(server.graphqlHandler)))

	router.Handle(
		"/api/v0/projects/{id}/usage-limits",
		server.withAuth(http.HandlerFunc(server.projectUsageLimitsHandler)),
	).Methods(http.MethodGet)

	referralsController := consoleapi.NewReferrals(logger, referralsService, service, mailService, server.config.ExternalAddress)
	referralsRouter := router.PathPrefix("/api/v0/referrals").Subrouter()
	referralsRouter.Handle("/tokens", server.withAuth(http.HandlerFunc(referralsController.GetTokens))).Methods(http.MethodGet)
	referralsRouter.HandleFunc("/register", referralsController.Register).Methods(http.MethodPost)

	authController := consoleapi.NewAuth(logger, service, mailService, server.cookieAuth, partners, server.config.ExternalAddress, config.LetUsKnowURL, config.TermsAndConditionsURL, config.ContactInfoURL)
	authRouter := router.PathPrefix("/api/v0/auth").Subrouter()
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.GetAccount))).Methods(http.MethodGet)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.UpdateAccount))).Methods(http.MethodPatch)
	authRouter.Handle("/account/change-email", server.withAuth(http.HandlerFunc(authController.ChangeEmail))).Methods(http.MethodPost)
	authRouter.Handle("/account/change-password", server.withAuth(http.HandlerFunc(authController.ChangePassword))).Methods(http.MethodPost)
	authRouter.Handle("/account/delete", server.withAuth(http.HandlerFunc(authController.DeleteAccount))).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", authController.Logout).Methods(http.MethodPost)
	authRouter.Handle("/token", server.rateLimiter.Limit(http.HandlerFunc(authController.Token))).Methods(http.MethodPost)
	authRouter.Handle("/register", server.rateLimiter.Limit(http.HandlerFunc(authController.Register))).Methods(http.MethodPost)
	authRouter.Handle("/forgot-password/{email}", server.rateLimiter.Limit(http.HandlerFunc(authController.ForgotPassword))).Methods(http.MethodPost)
	authRouter.Handle("/resend-email/{id}", server.rateLimiter.Limit(http.HandlerFunc(authController.ResendEmail))).Methods(http.MethodPost)

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
	paymentsRouter.HandleFunc("/paywall-enabled/{userId}", paymentController.PaywallEnabled).Methods(http.MethodGet)

	bucketsController := consoleapi.NewBuckets(logger, service)
	bucketsRouter := router.PathPrefix("/api/v0/buckets").Subrouter()
	bucketsRouter.Use(server.withAuth)
	bucketsRouter.HandleFunc("/bucket-names", bucketsController.AllBucketNames).Methods(http.MethodGet)

	if server.config.StaticDir != "" {
		router.HandleFunc("/activation/", server.accountActivationHandler)
		router.HandleFunc("/password-recovery/", server.passwordRecoveryHandler)
		router.HandleFunc("/cancel-password-recovery/", server.cancelPasswordRecoveryHandler)
		router.HandleFunc("/usage-report", server.bucketUsageReportHandler)
		router.PathPrefix("/static/").Handler(server.brotliMiddleware(http.StripPrefix("/static", fs)))
		router.PathPrefix("/").Handler(http.HandlerFunc(server.appHandler))
	}

	server.server = http.Server{
		Handler:        server.withRequest(router),
		MaxHeaderBytes: ContentLengthLimit.Int(),
	}

	return &server
}

// Run starts the server that host webapp and api endpoint.
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
		server.rateLimiter.Run(ctx)
		return nil
	})
	group.Go(func() error {
		defer cancel()
		err := server.server.Serve(server.listener)
		if errs2.IsCanceled(err) || errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		return err
	})

	return group.Wait()
}

// Close closes server and underlying listener.
func (server *Server) Close() error {
	return server.server.Close()
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	cspValues := []string{
		"default-src 'self'",
		"connect-src 'self' api.segment.io *.google-analytics.com " + server.config.GatewayCredentialsRequestURL,
		"frame-ancestors " + server.config.FrameAncestors,
		"frame-src 'self' *.stripe.com *.googletagmanager.com",
		"img-src 'self' data: *.customer.io *.googletagmanager.com *.google-analytics.com",
		"script-src 'sha256-wAqYV6m2PHGd1WDyFBnZmSoyfCK0jxFAns0vGbdiWUA=' 'self' *.stripe.com cdn.segment.com *.customer.io *.google-analytics.com *.googletagmanager.com",
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin") // Only expose the referring url when navigating around the satellite itself.

	var data struct {
		SatelliteName                   string
		SatelliteNodeURL                string
		SegmentIOPublicKey              string
		StripePublicKey                 string
		VerificationPageURL             string
		PartneredSatelliteNames         string
		GoogleTagManagerID              string
		DefaultProjectLimit             int
		GeneralRequestURL               string
		ProjectLimitsIncreaseRequestURL string
		GatewayCredentialsRequestURL    string
	}

	data.SatelliteName = server.config.SatelliteName
	data.SatelliteNodeURL = server.nodeURL.String()
	data.SegmentIOPublicKey = server.config.SegmentIOPublicKey
	data.StripePublicKey = server.stripePublicKey
	data.VerificationPageURL = server.config.VerificationPageURL
	data.PartneredSatelliteNames = server.config.PartneredSatelliteNames
	data.GoogleTagManagerID = server.config.GoogleTagManagerID
	data.DefaultProjectLimit = server.config.DefaultProjectLimit
	data.GeneralRequestURL = server.config.GeneralRequestURL
	data.ProjectLimitsIncreaseRequestURL = server.config.ProjectLimitsIncreaseRequestURL
	data.GatewayCredentialsRequestURL = server.config.GatewayCredentialsRequestURL

	if server.templates.index == nil {
		server.log.Error("index template is not set")
		return
	}

	if err := server.templates.index.Execute(w, data); err != nil {
		server.log.Error("index template could not be executed", zap.Error(err))
		return
	}
}

// authMiddlewareHandler performs initial authorization before every request.
func (server *Server) withAuth(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		var ctx context.Context

		defer mon.Task()(&ctx)(&err)

		ctxWithAuth := func(ctx context.Context) context.Context {
			token, err := server.cookieAuth.GetToken(r)
			if err != nil {
				return console.WithAuthFailure(ctx, err)
			}

			ctx = consoleauth.WithAPIKey(ctx, []byte(token))

			auth, err := server.service.Authorize(ctx)
			if err != nil {
				return console.WithAuthFailure(ctx, err)
			}

			return console.WithAuth(ctx, auth)
		}

		ctx = ctxWithAuth(r.Context())

		handler.ServeHTTP(w, r.Clone(ctx))
	})
}

// withRequest ensures the http request itself is reachable from the context.
func (server *Server) withRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.Clone(console.WithRequest(r.Context(), r)))
	})
}

// bucketUsageReportHandler generate bucket usage report page for project.
func (server *Server) bucketUsageReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	token, err := server.cookieAuth.GetToken(r)
	if err != nil {
		server.serveError(w, http.StatusUnauthorized)
		return
	}

	auth, err := server.service.Authorize(consoleauth.WithAPIKey(ctx, []byte(token)))
	if err != nil {
		server.serveError(w, http.StatusUnauthorized)
		return
	}

	ctx = console.WithAuth(ctx, auth)

	// parse query params
	projectID, err := uuid.FromString(r.URL.Query().Get("projectID"))
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

	since := time.Unix(sinceStamp, 0).UTC()
	before := time.Unix(beforeStamp, 0).UTC()

	server.log.Debug("querying bucket usage report",
		zap.Stringer("projectID", projectID),
		zap.Stringer("since", since),
		zap.Stringer("before", before))

	bucketRollups, err := server.service.GetBucketUsageRollups(ctx, projectID, since, before)
	if err != nil {
		server.log.Error("bucket usage report error", zap.Error(err))
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	if err = server.templates.usageReport.Execute(w, bucketRollups); err != nil {
		server.log.Error("bucket usage report error", zap.Error(err))
	}
}

// createRegistrationTokenHandler is web app http handler function.
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

	equality := subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("Authorization")),
		[]byte(server.config.AuthToken),
	)
	if equality != 1 {
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

// populatePromotionalCoupons is web app http handler function for populating promotional coupons.
func (server *Server) populatePromotionalCoupons(w http.ResponseWriter, r *http.Request) {
	var err error
	var ctx context.Context

	defer mon.Task()(&ctx)(&err)

	handleError := func(status int, err error) {
		w.WriteHeader(status)
		w.Header().Set(contentType, applicationJSON)

		var response struct {
			Error string `json:"error"`
		}

		response.Error = err.Error()

		if err := json.NewEncoder(w).Encode(response); err != nil {
			server.log.Error("failed to write json error response", zap.Error(Error.Wrap(err)))
		}
	}

	ctx = r.Context()

	equality := subtle.ConstantTimeCompare(
		[]byte(r.Header.Get("Authorization")),
		[]byte(server.config.AuthToken),
	)
	if equality != 1 {
		handleError(http.StatusUnauthorized, errs.New("unauthorized"))
		return
	}

	if err = server.service.Payments().PopulatePromotionalCoupons(ctx); err != nil {
		handleError(http.StatusInternalServerError, err)
		return
	}
}

// accountActivationHandler is web app http handler function.
func (server *Server) accountActivationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	activationToken := r.URL.Query().Get("token")

	err := server.service.ActivateAccount(ctx, activationToken)
	if err != nil {
		server.log.Error("activation: failed to activate account",
			zap.String("token", activationToken),
			zap.Error(err))

		if console.ErrEmailUsed.Has(err) {
			server.serveError(w, http.StatusConflict)
			return
		}

		if console.Error.Has(err) {
			server.serveError(w, http.StatusInternalServerError)
			return
		}

		server.serveError(w, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, server.config.AccountActivationRedirectURL, http.StatusTemporaryRedirect)
}

func (server *Server) passwordRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	recoveryToken := r.URL.Query().Get("token")
	if len(recoveryToken) == 0 {
		server.serveError(w, http.StatusNotFound)
		return
	}

	var data struct {
		SatelliteName string
	}

	data.SatelliteName = server.config.SatelliteName

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

		if err := server.templates.success.Execute(w, data); err != nil {
			server.log.Error("success reset password template could not be executed", zap.Error(Error.Wrap(err)))
			return
		}
	case http.MethodGet:
		if err := server.templates.resetPassword.Execute(w, data); err != nil {
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

// projectUsageLimitsHandler api handler for project usage limits.
func (server *Server) projectUsageLimitsHandler(w http.ResponseWriter, r *http.Request) {
	err := error(nil)
	ctx := r.Context()

	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	handleError := func(code int, err error) {
		w.WriteHeader(code)

		var jsonError struct {
			Error string `json:"error"`
		}

		// N.B. we are probably leaking internal details to the client
		jsonError.Error = err.Error()

		if err := json.NewEncoder(w).Encode(jsonError); err != nil {
			server.log.Error("error encoding project usage limits error", zap.Error(err))
		}
	}

	handleServiceError := func(err error) {
		switch {
		case console.ErrUnauthorized.Has(err):
			handleError(http.StatusUnauthorized, err)
		case accounting.ErrInvalidArgument.Has(err):
			handleError(http.StatusBadRequest, err)
		default:
			handleError(http.StatusInternalServerError, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		handleError(http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	projectID, err := uuid.FromString(idParam)
	if err != nil {
		handleError(http.StatusBadRequest, errs.New("invalid project id: %v", err))
		return
	}

	limits, err := server.service.GetProjectUsageLimits(ctx, projectID)
	if err != nil {
		handleServiceError(err)
		return
	}

	if err := json.NewEncoder(w).Encode(limits); err != nil {
		server.log.Error("error encoding project usage limits", zap.Error(err))
		return
	}
}

// graphqlHandler is graphql endpoint http handler function.
func (server *Server) graphqlHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	handleError := func(code int, err error) {
		w.WriteHeader(code)

		var jsonError struct {
			Error string `json:"error"`
		}

		jsonError.Error = err.Error()

		if err := json.NewEncoder(w).Encode(jsonError); err != nil {
			server.log.Error("error graphql error", zap.Error(err))
		}
	}

	w.Header().Set(contentType, applicationJSON)

	query, err := getQuery(w, r)
	if err != nil {
		handleError(http.StatusBadRequest, err)
		return
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

	getGqlError := func(err gqlerrors.FormattedError) error {
		if gerr, ok := err.OriginalError().(*gqlerrors.Error); ok {
			return gerr.OriginalError
		}

		return nil
	}

	parseConsoleError := func(err error) (int, error) {
		switch {
		case console.ErrUnauthorized.Has(err):
			return http.StatusUnauthorized, err
		case console.Error.Has(err):
			return http.StatusInternalServerError, err
		}

		return 0, nil
	}

	handleErrors := func(code int, errors gqlerrors.FormattedErrors) {
		w.WriteHeader(code)

		var jsonError struct {
			Errors []string `json:"errors"`
		}

		for _, err := range errors {
			jsonError.Errors = append(jsonError.Errors, err.Message)
		}

		if err := json.NewEncoder(w).Encode(jsonError); err != nil {
			server.log.Error("error graphql error", zap.Error(err))
		}
	}

	handleGraphqlErrors := func() {
		for _, err := range result.Errors {
			gqlErr := getGqlError(err)
			if gqlErr == nil {
				continue
			}

			code, err := parseConsoleError(gqlErr)
			if err != nil {
				handleError(code, err)
				return
			}
		}

		handleErrors(http.StatusOK, result.Errors)
	}

	if result.HasErrors() {
		handleGraphqlErrors()
		return
	}

	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		server.log.Error("error encoding grapql result", zap.Error(err))
		return
	}

	server.log.Debug(fmt.Sprintf("%s", result))
}

// serveError serves error static pages.
func (server *Server) serveError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)

	switch status {
	case http.StatusInternalServerError:
		err := server.templates.internalServerError.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse internalServerError template", zap.Error(Error.Wrap(err)))
		}
	case http.StatusNotFound:
		err := server.templates.notFound.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse pageNotFound template", zap.Error(Error.Wrap(err)))
		}
	case http.StatusConflict:
		err := server.templates.activated.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse already activated template", zap.Error(Error.Wrap(err)))
		}
	}
}

// seoHandler used to communicate with web crawlers and other web robots.
func (server *Server) seoHandler(w http.ResponseWriter, req *http.Request) {
	header := w.Header()

	header.Set(contentType, mime.TypeByExtension(".txt"))
	header.Set("X-Content-Type-Options", "nosniff")

	_, err := w.Write([]byte(server.config.SEO))
	if err != nil {
		server.log.Error(err.Error())
	}
}

// brotliMiddleware is used to compress static content using brotli to minify resources if browser support such decoding.
func (server *Server) brotliMiddleware(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=31536000")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		isBrotliSupported := strings.Contains(r.Header.Get("Accept-Encoding"), "br")
		if !isBrotliSupported {
			fn.ServeHTTP(w, r)
			return
		}

		info, err := os.Stat(server.config.StaticDir + strings.TrimPrefix(r.URL.Path, "/static") + ".br")
		if err != nil {
			fn.ServeHTTP(w, r)
			return
		}

		extension := filepath.Ext(info.Name()[:len(info.Name())-3])
		w.Header().Set(contentType, mime.TypeByExtension(extension))
		w.Header().Set("Content-Encoding", "br")

		newRequest := new(http.Request)
		*newRequest = *r
		newRequest.URL = new(url.URL)
		*newRequest.URL = *r.URL
		newRequest.URL.Path += ".br"

		fn.ServeHTTP(w, newRequest)
	})
}

// initializeTemplates is used to initialize all templates.
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
