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
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleauth"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/payments/paymentsconfig"
	"storj.io/storj/satellite/rewards"
)

const (
	contentType = "Content-Type"

	applicationJSON    = "application/json"
	applicationGraphql = "application/graphql"
)

var (
	// Error is satellite console error type.
	Error = errs.Class("consoleweb")

	mon = monkit.Package()
)

// Config contains configuration for console web server.
type Config struct {
	Address         string `help:"server address of the graphql api gateway and frontend app" devDefault:"127.0.0.1:0" releaseDefault:":10100"`
	StaticDir       string `help:"path to static resources" default:""`
	Watch           bool   `help:"whether to load templates on each request" default:"false" devDefault:"true"`
	ExternalAddress string `help:"external endpoint of the satellite if hosted" default:""`

	// TODO: remove after Vanguard release
	AuthToken       string `help:"auth token needed for access to registration token creation endpoint" default:"" testDefault:"very-secret-token"`
	AuthTokenSecret string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`

	ContactInfoURL                  string  `help:"url link to contacts page" default:"https://forum.storj.io"`
	FrameAncestors                  string  `help:"allow domains to embed the satellite in a frame, space separated" default:"tardigrade.io storj.io"`
	LetUsKnowURL                    string  `help:"url link to let us know page" default:"https://storjlabs.atlassian.net/servicedesk/customer/portals"`
	SEO                             string  `help:"used to communicate with web crawlers and other web robots" default:"User-agent: *\nDisallow: \nDisallow: /cgi-bin/"`
	SatelliteName                   string  `help:"used to display at web satellite console" default:"Storj"`
	SatelliteOperator               string  `help:"name of organization which set up satellite" default:"Storj Labs" `
	TermsAndConditionsURL           string  `help:"url link to terms and conditions page" default:"https://storj.io/storage-sla/"`
	AccountActivationRedirectURL    string  `help:"url link for account activation redirect" default:""`
	PartneredSatellites             SatList `help:"names and addresses of partnered satellites in JSON list format" default:"[[\"US1\",\"https://us1.storj.io\"],[\"EU1\",\"https://eu1.storj.io\"],[\"AP1\",\"https://ap1.storj.io\"]]"`
	GeneralRequestURL               string  `help:"url link to general request page" default:"https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000379291"`
	ProjectLimitsIncreaseRequestURL string  `help:"url link to project limit increase request page" default:"https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212"`
	GatewayCredentialsRequestURL    string  `help:"url link for gateway credentials requests" default:"https://auth.us1.storjshare.io" devDefault:""`
	IsBetaSatellite                 bool    `help:"indicates if satellite is in beta" default:"false"`
	BetaSatelliteFeedbackURL        string  `help:"url link for for beta satellite feedback" default:""`
	BetaSatelliteSupportURL         string  `help:"url link for for beta satellite support" default:""`
	DocumentationURL                string  `help:"url link to documentation" default:"https://docs.storj.io/"`
	CouponCodeBillingUIEnabled      bool    `help:"indicates if user is allowed to add coupon codes to account from billing" default:"false"`
	CouponCodeSignupUIEnabled       bool    `help:"indicates if user is allowed to add coupon codes to account from signup" default:"false"`
	FileBrowserFlowDisabled         bool    `help:"indicates if file browser flow is disabled" default:"false"`
	CSPEnabled                      bool    `help:"indicates if Content Security Policy is enabled" devDefault:"false" releaseDefault:"true"`
	LinksharingURL                  string  `help:"url link for linksharing requests" default:"https://link.us1.storjshare.io" devDefault:""`
	PathwayOverviewEnabled          bool    `help:"indicates if the overview onboarding step should render with pathways" default:"true"`
	NewProjectDashboard             bool    `help:"indicates if new project dashboard should be used" default:"false"`
	NewNavigation                   bool    `help:"indicates if new navigation structure should be rendered" default:"true"`
	NewObjectsFlow                  bool    `help:"indicates if new objects flow should be used" default:"true"`

	// RateLimit defines the configuration for the IP and userID rate limiters.
	RateLimit web.RateLimiterConfig

	console.Config
}

// SatList is a configuration value that contains a list of satellite names and addresses.
// Format should be [[name,address],[name,address],...] in valid JSON format.
//
// Can be used as a flag.
type SatList string

// Type implements pflag.Value.
func (SatList) Type() string { return "consoleweb.SatList" }

// String is required for pflag.Value.
func (sl *SatList) String() string {
	return string(*sl)
}

// Set does validation on the configured JSON, but does not actually transform it - it will be passed to the client as-is.
func (sl *SatList) Set(s string) error {
	satellites := make([][]string, 3)

	err := json.Unmarshal([]byte(s), &satellites)
	if err != nil {
		return err
	}

	for _, sat := range satellites {
		if len(sat) != 2 {
			return errs.New("Could not parse satellite list config. Each satellite in the config must have two values: [name, address]")
		}
	}

	*sl = SatList(s)
	return nil
}

// Server represents console web server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config      Config
	service     *console.Service
	mailService *mailservice.Service
	partners    *rewards.PartnersService
	analytics   *analytics.Service

	listener          net.Listener
	server            http.Server
	cookieAuth        *consolewebauth.CookieAuth
	ipRateLimiter     *web.RateLimiter
	userIDRateLimiter *web.RateLimiter
	nodeURL           storj.NodeURL

	stripePublicKey string

	pricing paymentsconfig.PricingValues

	schema graphql.Schema

	templatesCache *templates
}

type templates struct {
	index               *template.Template
	notFound            *template.Template
	internalServerError *template.Template
	usageReport         *template.Template
}

// NewServer creates new instance of console server.
func NewServer(logger *zap.Logger, config Config, service *console.Service, mailService *mailservice.Service, partners *rewards.PartnersService, analytics *analytics.Service, listener net.Listener, stripePublicKey string, pricing paymentsconfig.PricingValues, nodeURL storj.NodeURL) *Server {
	server := Server{
		log:               logger,
		config:            config,
		listener:          listener,
		service:           service,
		mailService:       mailService,
		partners:          partners,
		analytics:         analytics,
		stripePublicKey:   stripePublicKey,
		ipRateLimiter:     web.NewIPRateLimiter(config.RateLimit),
		userIDRateLimiter: NewUserIDRateLimiter(config.RateLimit),
		nodeURL:           nodeURL,
		pricing:           pricing,
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
	router.HandleFunc("/robots.txt", server.seoHandler)

	router.Handle("/api/v0/graphql", server.withAuth(http.HandlerFunc(server.graphqlHandler)))

	usageLimitsController := consoleapi.NewUsageLimits(logger, service)
	router.Handle(
		"/api/v0/projects/{id}/usage-limits",
		server.withAuth(http.HandlerFunc(usageLimitsController.ProjectUsageLimits)),
	).Methods(http.MethodGet)
	router.Handle(
		"/api/v0/projects/usage-limits",
		server.withAuth(http.HandlerFunc(usageLimitsController.TotalUsageLimits)),
	).Methods(http.MethodGet)
	router.Handle(
		"/api/v0/projects/{id}/daily-usage",
		server.withAuth(http.HandlerFunc(usageLimitsController.DailyUsage)),
	).Methods(http.MethodGet)

	authController := consoleapi.NewAuth(logger, service, mailService, server.cookieAuth, partners, server.analytics, server.config.ExternalAddress, config.LetUsKnowURL, config.TermsAndConditionsURL, config.ContactInfoURL)
	authRouter := router.PathPrefix("/api/v0/auth").Subrouter()
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.GetAccount))).Methods(http.MethodGet)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.UpdateAccount))).Methods(http.MethodPatch)
	authRouter.Handle("/account/change-email", server.withAuth(http.HandlerFunc(authController.ChangeEmail))).Methods(http.MethodPost)
	authRouter.Handle("/account/change-password", server.withAuth(http.HandlerFunc(authController.ChangePassword))).Methods(http.MethodPost)
	authRouter.Handle("/account/delete", server.withAuth(http.HandlerFunc(authController.DeleteAccount))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/enable", server.withAuth(http.HandlerFunc(authController.EnableUserMFA))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/disable", server.withAuth(http.HandlerFunc(authController.DisableUserMFA))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/generate-secret-key", server.withAuth(http.HandlerFunc(authController.GenerateMFASecretKey))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/generate-recovery-codes", server.withAuth(http.HandlerFunc(authController.GenerateMFARecoveryCodes))).Methods(http.MethodPost)
	authRouter.HandleFunc("/logout", authController.Logout).Methods(http.MethodPost)
	authRouter.Handle("/token", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Token))).Methods(http.MethodPost)
	authRouter.Handle("/register", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Register))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/forgot-password/{email}", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ForgotPassword))).Methods(http.MethodPost)
	authRouter.Handle("/resend-email/{email}", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResendEmail))).Methods(http.MethodPost)
	authRouter.Handle("/reset-password", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResetPassword))).Methods(http.MethodPost)

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
	paymentsRouter.Handle("/coupon/apply", server.userIDRateLimiter.Limit(http.HandlerFunc(paymentController.ApplyCouponCode))).Methods(http.MethodPatch)
	paymentsRouter.HandleFunc("/coupon", paymentController.GetCoupon).Methods(http.MethodGet)

	bucketsController := consoleapi.NewBuckets(logger, service)
	bucketsRouter := router.PathPrefix("/api/v0/buckets").Subrouter()
	bucketsRouter.Use(server.withAuth)
	bucketsRouter.HandleFunc("/bucket-names", bucketsController.AllBucketNames).Methods(http.MethodGet)

	apiKeysController := consoleapi.NewAPIKeys(logger, service)
	apiKeysRouter := router.PathPrefix("/api/v0/api-keys").Subrouter()
	apiKeysRouter.Use(server.withAuth)
	apiKeysRouter.HandleFunc("/delete-by-name", apiKeysController.DeleteByNameAndProjectID).Methods(http.MethodDelete)

	analyticsController := consoleapi.NewAnalytics(logger, service, server.analytics)
	analyticsRouter := router.PathPrefix("/api/v0/analytics").Subrouter()
	analyticsRouter.Use(server.withAuth)
	analyticsRouter.HandleFunc("/event", analyticsController.EventTriggered).Methods(http.MethodPost)

	if server.config.StaticDir != "" {
		router.HandleFunc("/activation/", server.accountActivationHandler)
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

	_, err = server.loadTemplates()
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
		server.ipRateLimiter.Run(ctx)
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

	if server.config.CSPEnabled {
		cspValues := []string{
			"default-src 'self'",
			"connect-src 'self' *.tardigradeshare.io *.storjshare.io " + server.config.GatewayCredentialsRequestURL,
			"frame-ancestors " + server.config.FrameAncestors,
			"frame-src 'self' *.stripe.com https://www.google.com/recaptcha/ https://recaptcha.google.com/recaptcha/",
			"img-src 'self' data: *.tardigradeshare.io *.storjshare.io",
			// Those are hashes of charts custom tooltip inline styles. They have to be updated if styles are updated.
			"style-src 'unsafe-hashes' 'sha256-7mY2NKmZ4PuyjGUa4FYC5u36SxXdoUM/zxrlr3BEToo=' 'sha256-PRTMwLUW5ce9tdiUrVCGKqj6wPeuOwGogb1pmyuXhgI=' 'sha256-kwpt3lQZ21rs4cld7/uEm9qI5yAbjYzx+9FGm/XmwNU=' 'self'",
			"media-src 'self' *.tardigradeshare.io *.storjshare.io",
			"script-src 'sha256-wAqYV6m2PHGd1WDyFBnZmSoyfCK0jxFAns0vGbdiWUA=' 'self' *.stripe.com https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/",
		}

		header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin") // Only expose the referring url when navigating around the satellite itself.

	var data struct {
		ExternalAddress                 string
		SatelliteName                   string
		SatelliteNodeURL                string
		StripePublicKey                 string
		PartneredSatellites             string
		DefaultProjectLimit             int
		GeneralRequestURL               string
		ProjectLimitsIncreaseRequestURL string
		GatewayCredentialsRequestURL    string
		IsBetaSatellite                 bool
		BetaSatelliteFeedbackURL        string
		BetaSatelliteSupportURL         string
		DocumentationURL                string
		CouponCodeBillingUIEnabled      bool
		CouponCodeSignupUIEnabled       bool
		FileBrowserFlowDisabled         bool
		LinksharingURL                  string
		PathwayOverviewEnabled          bool
		StorageTBPrice                  string
		EgressTBPrice                   string
		SegmentPrice                    string
		RecaptchaEnabled                bool
		RecaptchaSiteKey                string
		NewProjectDashboard             bool
		DefaultPaidStorageLimit         memory.Size
		DefaultPaidBandwidthLimit       memory.Size
		NewNavigation                   bool
		NewObjectsFlow                  bool
	}

	data.ExternalAddress = server.config.ExternalAddress
	data.SatelliteName = server.config.SatelliteName
	data.SatelliteNodeURL = server.nodeURL.String()
	data.StripePublicKey = server.stripePublicKey
	data.PartneredSatellites = string(server.config.PartneredSatellites)
	data.DefaultProjectLimit = server.config.DefaultProjectLimit
	data.GeneralRequestURL = server.config.GeneralRequestURL
	data.ProjectLimitsIncreaseRequestURL = server.config.ProjectLimitsIncreaseRequestURL
	data.GatewayCredentialsRequestURL = server.config.GatewayCredentialsRequestURL
	data.IsBetaSatellite = server.config.IsBetaSatellite
	data.BetaSatelliteFeedbackURL = server.config.BetaSatelliteFeedbackURL
	data.BetaSatelliteSupportURL = server.config.BetaSatelliteSupportURL
	data.DocumentationURL = server.config.DocumentationURL
	data.CouponCodeBillingUIEnabled = server.config.CouponCodeBillingUIEnabled
	data.CouponCodeSignupUIEnabled = server.config.CouponCodeSignupUIEnabled
	data.FileBrowserFlowDisabled = server.config.FileBrowserFlowDisabled
	data.LinksharingURL = server.config.LinksharingURL
	data.PathwayOverviewEnabled = server.config.PathwayOverviewEnabled
	data.DefaultPaidStorageLimit = server.config.UsageLimits.Storage.Paid
	data.DefaultPaidBandwidthLimit = server.config.UsageLimits.Bandwidth.Paid
	data.StorageTBPrice = server.pricing.StorageTBPrice
	data.EgressTBPrice = server.pricing.EgressTBPrice
	data.SegmentPrice = server.pricing.SegmentPrice
	data.RecaptchaEnabled = server.config.Recaptcha.Enabled
	data.RecaptchaSiteKey = server.config.Recaptcha.SiteKey
	data.NewProjectDashboard = server.config.NewProjectDashboard
	data.NewNavigation = server.config.NewNavigation
	data.NewObjectsFlow = server.config.NewObjectsFlow

	templates, err := server.loadTemplates()
	if err != nil || templates.index == nil {
		server.log.Error("unable to load templates", zap.Error(err))
		fmt.Fprintf(w, "Unable to load templates. See whether satellite UI has been built.")
		return
	}

	if err := templates.index.Execute(w, data); err != nil {
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

	templates, err := server.loadTemplates()
	if err != nil {
		server.log.Error("unable to load templates", zap.Error(err))
		return
	}
	if err = templates.usageReport.Execute(w, bucketRollups); err != nil {
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

// accountActivationHandler is web app http handler function.
func (server *Server) accountActivationHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	activationToken := r.URL.Query().Get("token")

	token, err := server.service.ActivateAccount(ctx, activationToken)
	if err != nil {
		server.log.Error("activation: failed to activate account",
			zap.String("token", activationToken),
			zap.Error(err))

		if console.ErrEmailUsed.Has(err) {
			http.Redirect(w, r, server.config.ExternalAddress+"login?activated=false", http.StatusTemporaryRedirect)
			return
		}

		if console.Error.Has(err) {
			server.serveError(w, http.StatusInternalServerError)
			return
		}

		server.serveError(w, http.StatusNotFound)
		return
	}

	server.cookieAuth.SetTokenCookie(w, token)

	http.Redirect(w, r, server.config.ExternalAddress, http.StatusTemporaryRedirect)
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
		var gerr *gqlerrors.Error
		if errors.As(err.OriginalError(), &gerr) {
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
		templates, err := server.loadTemplates()
		if err != nil {
			server.log.Error("unable to load templates", zap.Error(err))
			return
		}
		err = templates.internalServerError.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse internalServerError template", zap.Error(err))
		}
	case http.StatusNotFound:
		templates, err := server.loadTemplates()
		if err != nil {
			server.log.Error("unable to load templates", zap.Error(err))
			return
		}
		err = templates.notFound.Execute(w, nil)
		if err != nil {
			server.log.Error("cannot parse pageNotFound template", zap.Error(err))
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

// loadTemplates is used to initialize all templates.
func (server *Server) loadTemplates() (_ *templates, err error) {
	if server.config.Watch {
		return server.parseTemplates()
	}

	if server.templatesCache != nil {
		return server.templatesCache, nil
	}

	templates, err := server.parseTemplates()
	if err != nil {
		return nil, Error.Wrap(err)
	}

	server.templatesCache = templates
	return server.templatesCache, nil
}

func (server *Server) parseTemplates() (_ *templates, err error) {
	var t templates

	t.index, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "dist", "index.html"))
	if err != nil {
		server.log.Error("dist folder is not generated. use 'npm run build' command", zap.Error(err))
		// Loading index is optional.
	}

	t.usageReport, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "reports", "usageReport.html"))
	if err != nil {
		return &t, Error.Wrap(err)
	}

	t.notFound, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "errors", "404.html"))
	if err != nil {
		return &t, Error.Wrap(err)
	}

	t.internalServerError, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "errors", "500.html"))
	if err != nil {
		return &t, Error.Wrap(err)
	}

	return &t, nil
}

// NewUserIDRateLimiter constructs a RateLimiter that limits based on user ID.
func NewUserIDRateLimiter(config web.RateLimiterConfig) *web.RateLimiter {
	return web.NewRateLimiter(config, func(r *http.Request) (string, error) {
		auth, err := console.GetAuth(r.Context())
		if err != nil {
			return "", err
		}
		return auth.User.ID.String(), nil
	})
}
