// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleweb

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
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
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/storj"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consoleql"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/payments/paymentsconfig"
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

	AuthToken       string `help:"auth token needed for access to registration token creation endpoint" default:"" testDefault:"very-secret-token"`
	AuthTokenSecret string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`

	ContactInfoURL                  string     `help:"url link to contacts page" default:"https://forum.storj.io"`
	FrameAncestors                  string     `help:"allow domains to embed the satellite in a frame, space separated" default:"tardigrade.io storj.io"`
	LetUsKnowURL                    string     `help:"url link to let us know page" default:"https://storjlabs.atlassian.net/servicedesk/customer/portals"`
	SEO                             string     `help:"used to communicate with web crawlers and other web robots" default:"User-agent: *\nDisallow: \nDisallow: /cgi-bin/"`
	SatelliteName                   string     `help:"used to display at web satellite console" default:"Storj"`
	SatelliteOperator               string     `help:"name of organization which set up satellite" default:"Storj Labs" `
	TermsAndConditionsURL           string     `help:"url link to terms and conditions page" default:"https://www.storj.io/terms-of-service/"`
	AccountActivationRedirectURL    string     `help:"url link for account activation redirect" default:""`
	PartneredSatellites             Satellites `help:"names and addresses of partnered satellites in JSON list format" default:"[{\"name\":\"US1\",\"address\":\"https://us1.storj.io\"},{\"name\":\"EU1\",\"address\":\"https://eu1.storj.io\"},{\"name\":\"AP1\",\"address\":\"https://ap1.storj.io\"}]"`
	GeneralRequestURL               string     `help:"url link to general request page" default:"https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000379291"`
	ProjectLimitsIncreaseRequestURL string     `help:"url link to project limit increase request page" default:"https://supportdcs.storj.io/hc/en-us/requests/new?ticket_form_id=360000683212"`
	GatewayCredentialsRequestURL    string     `help:"url link for gateway credentials requests" default:"https://auth.storjshare.io" devDefault:"http://localhost:8000"`
	IsBetaSatellite                 bool       `help:"indicates if satellite is in beta" default:"false"`
	BetaSatelliteFeedbackURL        string     `help:"url link for for beta satellite feedback" default:""`
	BetaSatelliteSupportURL         string     `help:"url link for for beta satellite support" default:""`
	DocumentationURL                string     `help:"url link to documentation" default:"https://docs.storj.io/"`
	CouponCodeBillingUIEnabled      bool       `help:"indicates if user is allowed to add coupon codes to account from billing" default:"false"`
	CouponCodeSignupUIEnabled       bool       `help:"indicates if user is allowed to add coupon codes to account from signup" default:"false"`
	FileBrowserFlowDisabled         bool       `help:"indicates if file browser flow is disabled" default:"false"`
	CSPEnabled                      bool       `help:"indicates if Content Security Policy is enabled" devDefault:"false" releaseDefault:"true"`
	LinksharingURL                  string     `help:"url link for linksharing requests" default:"https://link.storjshare.io" devDefault:"http://localhost:8001"`
	PathwayOverviewEnabled          bool       `help:"indicates if the overview onboarding step should render with pathways" default:"true"`
	AllProjectsDashboard            bool       `help:"indicates if all projects dashboard should be used" default:"false"`
	GeneratedAPIEnabled             bool       `help:"indicates if generated console api should be used" default:"false"`
	OptionalSignupSuccessURL        string     `help:"optional url to external registration success page" default:""`
	HomepageURL                     string     `help:"url link to storj.io homepage" default:"https://www.storj.io"`
	NativeTokenPaymentsEnabled      bool       `help:"indicates if storj native token payments system is enabled" default:"false"`
	PricingPackagesEnabled          bool       `help:"whether to allow purchasing pricing packages" default:"false" devDefault:"true"`

	OauthCodeExpiry         time.Duration `help:"how long oauth authorization codes are issued for" default:"10m"`
	OauthAccessTokenExpiry  time.Duration `help:"how long oauth access tokens are issued for" default:"24h"`
	OauthRefreshTokenExpiry time.Duration `help:"how long oauth refresh tokens are issued for" default:"720h"`

	// RateLimit defines the configuration for the IP and userID rate limiters.
	RateLimit web.RateLimiterConfig

	ABTesting abtesting.Config

	console.Config
}

// Server represents console web server.
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config      Config
	service     *console.Service
	mailService *mailservice.Service
	analytics   *analytics.Service
	abTesting   *abtesting.Service

	listener          net.Listener
	server            http.Server
	cookieAuth        *consolewebauth.CookieAuth
	ipRateLimiter     *web.RateLimiter
	userIDRateLimiter *web.RateLimiter
	nodeURL           storj.NodeURL

	stripePublicKey string

	packagePlans paymentsconfig.PackagePlans

	schema graphql.Schema

	errorTemplate *template.Template
}

// apiAuth exposes methods to control authentication process for each generated API endpoint.
type apiAuth struct {
	server *Server
}

// IsAuthenticated checks if request is performed with all needed authorization credentials.
func (a *apiAuth) IsAuthenticated(ctx context.Context, r *http.Request, isCookieAuth, isKeyAuth bool) (_ context.Context, err error) {
	if isCookieAuth && isKeyAuth {
		ctx, err = a.cookieAuth(ctx, r)
		if err != nil {
			ctx, err = a.keyAuth(ctx, r)
			if err != nil {
				return nil, err
			}
		}
	} else if isCookieAuth {
		ctx, err = a.cookieAuth(ctx, r)
		if err != nil {
			return nil, err
		}
	} else if isKeyAuth {
		ctx, err = a.keyAuth(ctx, r)
		if err != nil {
			return nil, err
		}
	}

	return ctx, nil
}

// cookieAuth returns an authenticated context by session cookie.
func (a *apiAuth) cookieAuth(ctx context.Context, r *http.Request) (context.Context, error) {
	tokenInfo, err := a.server.cookieAuth.GetToken(r)
	if err != nil {
		return nil, err
	}

	return a.server.service.TokenAuth(ctx, tokenInfo.Token, time.Now())
}

// cookieAuth returns an authenticated context by api key.
func (a *apiAuth) keyAuth(ctx context.Context, r *http.Request) (context.Context, error) {
	authToken := r.Header.Get("Authorization")
	split := strings.Split(authToken, "Bearer ")
	if len(split) != 2 {
		return ctx, errs.New("authorization key format is incorrect. Should be 'Bearer <key>'")
	}

	return a.server.service.KeyAuth(ctx, split[1], time.Now())
}

// RemoveAuthCookie indicates to the client that the authentication cookie should be removed.
func (a *apiAuth) RemoveAuthCookie(w http.ResponseWriter) {
	a.server.cookieAuth.RemoveTokenCookie(w)
}

// NewServer creates new instance of console server.
func NewServer(logger *zap.Logger, config Config, service *console.Service, oidcService *oidc.Service, mailService *mailservice.Service, analytics *analytics.Service, abTesting *abtesting.Service, accountFreezeService *console.AccountFreezeService, listener net.Listener, stripePublicKey string, nodeURL storj.NodeURL, packagePlans paymentsconfig.PackagePlans) *Server {
	server := Server{
		log:               logger,
		config:            config,
		listener:          listener,
		service:           service,
		mailService:       mailService,
		analytics:         analytics,
		abTesting:         abTesting,
		stripePublicKey:   stripePublicKey,
		ipRateLimiter:     web.NewIPRateLimiter(config.RateLimit, logger),
		userIDRateLimiter: NewUserIDRateLimiter(config.RateLimit, logger),
		nodeURL:           nodeURL,
		packagePlans:      packagePlans,
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
	// N.B. This middleware has to be the first one because it has to be called
	// the earliest in the HTTP chain.
	router.Use(newTraceRequestMiddleware(logger, router))

	if server.config.GeneratedAPIEnabled {
		consoleapi.NewProjectManagement(logger, mon, server.service, router, &apiAuth{&server})
		consoleapi.NewAPIKeyManagement(logger, mon, server.service, router, &apiAuth{&server})
		consoleapi.NewUserManagement(logger, mon, server.service, router, &apiAuth{&server})
	}

	projectsController := consoleapi.NewProjects(logger, service)
	router.Handle(
		"/api/v0/projects/{id}/salt",
		server.withAuth(http.HandlerFunc(projectsController.GetSalt)),
	).Methods(http.MethodGet)

	router.HandleFunc("/api/v0/config", server.frontendConfigHandler)

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

	authController := consoleapi.NewAuth(logger, service, accountFreezeService, mailService, server.cookieAuth, server.analytics, config.SatelliteName, server.config.ExternalAddress, config.LetUsKnowURL, config.TermsAndConditionsURL, config.ContactInfoURL, config.GeneralRequestURL)
	authRouter := router.PathPrefix("/api/v0/auth").Subrouter()
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.GetAccount))).Methods(http.MethodGet)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.UpdateAccount))).Methods(http.MethodPatch)
	authRouter.Handle("/account/change-email", server.withAuth(http.HandlerFunc(authController.ChangeEmail))).Methods(http.MethodPost)
	authRouter.Handle("/account/change-password", server.withAuth(server.userIDRateLimiter.Limit(http.HandlerFunc(authController.ChangePassword)))).Methods(http.MethodPost)
	authRouter.Handle("/account/freezestatus", server.withAuth(http.HandlerFunc(authController.GetFreezeStatus))).Methods(http.MethodGet)
	authRouter.Handle("/account/settings", server.withAuth(http.HandlerFunc(authController.GetUserSettings))).Methods(http.MethodGet)
	authRouter.Handle("/account/settings", server.withAuth(http.HandlerFunc(authController.SetUserSettings))).Methods(http.MethodPatch)
	authRouter.Handle("/account/onboarding", server.withAuth(http.HandlerFunc(authController.SetOnboardingStatus))).Methods(http.MethodPatch)
	authRouter.Handle("/account/delete", server.withAuth(http.HandlerFunc(authController.DeleteAccount))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/enable", server.withAuth(http.HandlerFunc(authController.EnableUserMFA))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/disable", server.withAuth(http.HandlerFunc(authController.DisableUserMFA))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/generate-secret-key", server.withAuth(http.HandlerFunc(authController.GenerateMFASecretKey))).Methods(http.MethodPost)
	authRouter.Handle("/mfa/generate-recovery-codes", server.withAuth(http.HandlerFunc(authController.GenerateMFARecoveryCodes))).Methods(http.MethodPost)
	authRouter.Handle("/logout", server.withAuth(http.HandlerFunc(authController.Logout))).Methods(http.MethodPost)
	authRouter.Handle("/token", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Token))).Methods(http.MethodPost)
	authRouter.Handle("/token-by-api-key", server.ipRateLimiter.Limit(http.HandlerFunc(authController.TokenByAPIKey))).Methods(http.MethodPost)
	authRouter.Handle("/register", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Register))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/forgot-password", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ForgotPassword))).Methods(http.MethodPost)
	authRouter.Handle("/resend-email/{email}", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResendEmail))).Methods(http.MethodPost)
	authRouter.Handle("/reset-password", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResetPassword))).Methods(http.MethodPost)
	authRouter.Handle("/refresh-session", server.withAuth(http.HandlerFunc(authController.RefreshSession))).Methods(http.MethodPost)

	if config.ABTesting.Enabled {
		abController := consoleapi.NewABTesting(logger, abTesting)
		abRouter := router.PathPrefix("/api/v0/ab").Subrouter()
		abRouter.Handle("/values", server.withAuth(http.HandlerFunc(abController.GetABValues))).Methods(http.MethodGet)
		abRouter.Handle("/hit/{action}", server.withAuth(http.HandlerFunc(abController.SendHit))).Methods(http.MethodPost)
	}

	paymentController := consoleapi.NewPayments(logger, service, accountFreezeService, packagePlans)
	paymentsRouter := router.PathPrefix("/api/v0/payments").Subrouter()
	paymentsRouter.Use(server.withAuth)
	paymentsRouter.Handle("/cards", server.userIDRateLimiter.Limit(http.HandlerFunc(paymentController.AddCreditCard))).Methods(http.MethodPost)
	paymentsRouter.HandleFunc("/cards", paymentController.MakeCreditCardDefault).Methods(http.MethodPatch)
	paymentsRouter.HandleFunc("/cards", paymentController.ListCreditCards).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/cards/{cardId}", paymentController.RemoveCreditCard).Methods(http.MethodDelete)
	paymentsRouter.HandleFunc("/account/charges", paymentController.ProjectsCharges).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/account/balance", paymentController.AccountBalance).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/account", paymentController.SetupAccount).Methods(http.MethodPost)
	paymentsRouter.HandleFunc("/wallet", paymentController.GetWallet).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/wallet", paymentController.ClaimWallet).Methods(http.MethodPost)
	paymentsRouter.HandleFunc("/wallet/payments", paymentController.WalletPayments).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/billing-history", paymentController.BillingHistory).Methods(http.MethodGet)
	paymentsRouter.Handle("/coupon/apply", server.userIDRateLimiter.Limit(http.HandlerFunc(paymentController.ApplyCouponCode))).Methods(http.MethodPatch)
	paymentsRouter.HandleFunc("/coupon", paymentController.GetCoupon).Methods(http.MethodGet)
	paymentsRouter.HandleFunc("/pricing", paymentController.GetProjectUsagePriceModel).Methods(http.MethodGet)
	if config.PricingPackagesEnabled {
		paymentsRouter.HandleFunc("/purchase-package", paymentController.PurchasePackage).Methods(http.MethodPost)
		paymentsRouter.HandleFunc("/package-available", paymentController.PackageAvailable).Methods(http.MethodGet)
	}

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
	analyticsRouter.HandleFunc("/page", analyticsController.PageEventTriggered).Methods(http.MethodPost)

	if server.config.StaticDir != "" {
		oidc := oidc.NewEndpoint(
			server.nodeURL, server.config.ExternalAddress,
			logger, oidcService, service,
			server.config.OauthCodeExpiry, server.config.OauthAccessTokenExpiry, server.config.OauthRefreshTokenExpiry,
		)

		router.HandleFunc("/.well-known/openid-configuration", oidc.WellKnownConfiguration)
		router.Handle("/oauth/v2/authorize", server.withAuth(http.HandlerFunc(oidc.AuthorizeUser))).Methods(http.MethodPost)
		router.Handle("/oauth/v2/tokens", server.ipRateLimiter.Limit(http.HandlerFunc(oidc.Tokens))).Methods(http.MethodPost)
		router.Handle("/oauth/v2/userinfo", server.ipRateLimiter.Limit(http.HandlerFunc(oidc.UserInfo))).Methods(http.MethodGet)
		router.Handle("/oauth/v2/clients/{id}", server.withAuth(http.HandlerFunc(oidc.GetClient))).Methods(http.MethodGet)

		fs := http.FileServer(http.Dir(server.config.StaticDir))
		router.PathPrefix("/static/").Handler(server.brotliMiddleware(http.StripPrefix("/static", fs)))

		// These paths previously required a trailing slash, so we support both forms for now
		slashRouter := router.NewRoute().Subrouter()
		slashRouter.StrictSlash(true)
		slashRouter.HandleFunc("/activation", server.accountActivationHandler)
		slashRouter.HandleFunc("/cancel-password-recovery", server.cancelPasswordRecoveryHandler)

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

	_, err = server.loadErrorTemplate()
	if err != nil {
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
			"script-src 'sha256-wAqYV6m2PHGd1WDyFBnZmSoyfCK0jxFAns0vGbdiWUA=' 'self' *.stripe.com https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/ https://hcaptcha.com *.hcaptcha.com",
			"connect-src 'self' *.tardigradeshare.io *.storjshare.io https://hcaptcha.com *.hcaptcha.com " + server.config.GatewayCredentialsRequestURL,
			"frame-ancestors " + server.config.FrameAncestors,
			"frame-src 'self' *.stripe.com https://www.google.com/recaptcha/ https://recaptcha.google.com/recaptcha/ https://hcaptcha.com *.hcaptcha.com",
			"img-src 'self' data: blob: *.tardigradeshare.io *.storjshare.io",
			// Those are hashes of charts custom tooltip inline styles. They have to be updated if styles are updated.
			"style-src 'unsafe-hashes' 'sha256-7mY2NKmZ4PuyjGUa4FYC5u36SxXdoUM/zxrlr3BEToo=' 'sha256-PRTMwLUW5ce9tdiUrVCGKqj6wPeuOwGogb1pmyuXhgI=' 'sha256-kwpt3lQZ21rs4cld7/uEm9qI5yAbjYzx+9FGm/XmwNU=' 'sha256-Qf4xqtNKtDLwxce6HLtD5Y6BWpOeR7TnDpNSo+Bhb3s=' 'self' https://hcaptcha.com *.hcaptcha.com",
			"media-src 'self' blob: *.tardigradeshare.io *.storjshare.io",
		}

		header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin") // Only expose the referring url when navigating around the satellite itself.

	path := filepath.Join(server.config.StaticDir, "dist", "index.html")
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			server.log.Error("index.html was not generated. run 'npm run build' in the "+server.config.StaticDir+" directory", zap.Error(err))
		} else {
			server.log.Error("error loading index.html", zap.String("path", path), zap.Error(err))
		}
		// Loading index is optional.
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			server.log.Error("error closing index.html", zap.String("path", path), zap.Error(err))
		}
	}()

	info, err := file.Stat()
	if err != nil {
		server.log.Error("failed to retrieve index.html file info", zap.Error(err))
		return
	}

	http.ServeContent(w, r, path, info.ModTime(), file)
}

// withAuth performs initial authorization before every request.
func (server *Server) withAuth(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		defer mon.Task()(&ctx)(&err)

		defer func() {
			if err != nil {
				web.ServeJSONError(server.log, w, http.StatusUnauthorized, console.ErrUnauthorized.Wrap(err))
				server.cookieAuth.RemoveTokenCookie(w)
			}
		}()

		tokenInfo, err := server.cookieAuth.GetToken(r)
		if err != nil {
			return
		}

		newCtx, err := server.service.TokenAuth(ctx, tokenInfo.Token, time.Now())
		if err != nil {
			return
		}
		ctx = newCtx

		handler.ServeHTTP(w, r.Clone(ctx))
	})
}

// withRequest ensures the http request itself is reachable from the context.
func (server *Server) withRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.Clone(console.WithRequest(r.Context(), r)))
	})
}

// frontendConfigHandler handles sending the frontend config to the client.
func (server *Server) frontendConfigHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)
	w.Header().Set(contentType, applicationJSON)

	cfg := FrontendConfig{
		ExternalAddress:                 server.config.ExternalAddress,
		SatelliteName:                   server.config.SatelliteName,
		SatelliteNodeURL:                server.nodeURL.String(),
		StripePublicKey:                 server.stripePublicKey,
		PartneredSatellites:             server.config.PartneredSatellites,
		DefaultProjectLimit:             server.config.DefaultProjectLimit,
		GeneralRequestURL:               server.config.GeneralRequestURL,
		ProjectLimitsIncreaseRequestURL: server.config.ProjectLimitsIncreaseRequestURL,
		GatewayCredentialsRequestURL:    server.config.GatewayCredentialsRequestURL,
		IsBetaSatellite:                 server.config.IsBetaSatellite,
		BetaSatelliteFeedbackURL:        server.config.BetaSatelliteFeedbackURL,
		BetaSatelliteSupportURL:         server.config.BetaSatelliteSupportURL,
		DocumentationURL:                server.config.DocumentationURL,
		CouponCodeBillingUIEnabled:      server.config.CouponCodeBillingUIEnabled,
		CouponCodeSignupUIEnabled:       server.config.CouponCodeSignupUIEnabled,
		FileBrowserFlowDisabled:         server.config.FileBrowserFlowDisabled,
		LinksharingURL:                  server.config.LinksharingURL,
		PathwayOverviewEnabled:          server.config.PathwayOverviewEnabled,
		DefaultPaidStorageLimit:         server.config.UsageLimits.Storage.Paid,
		DefaultPaidBandwidthLimit:       server.config.UsageLimits.Bandwidth.Paid,
		Captcha:                         server.config.Captcha,
		AllProjectsDashboard:            server.config.AllProjectsDashboard,
		InactivityTimerEnabled:          server.config.Session.InactivityTimerEnabled,
		InactivityTimerDuration:         server.config.Session.InactivityTimerDuration,
		InactivityTimerViewerEnabled:    server.config.Session.InactivityTimerViewerEnabled,
		OptionalSignupSuccessURL:        server.config.OptionalSignupSuccessURL,
		HomepageURL:                     server.config.HomepageURL,
		NativeTokenPaymentsEnabled:      server.config.NativeTokenPaymentsEnabled,
		PasswordMinimumLength:           console.PasswordMinimumLength,
		PasswordMaximumLength:           console.PasswordMaximumLength,
		ABTestingEnabled:                server.config.ABTesting.Enabled,
		PricingPackagesEnabled:          server.config.PricingPackagesEnabled,
	}

	err := json.NewEncoder(w).Encode(&cfg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		server.log.Error("failed to write frontend config", zap.Error(err))
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

	user, err := server.service.ActivateAccount(ctx, activationToken)
	if err != nil {
		if console.ErrTokenInvalid.Has(err) {
			server.log.Debug("account activation",
				zap.String("token", activationToken),
				zap.Error(err),
			)
			server.serveError(w, http.StatusBadRequest)
			return
		}

		if console.ErrTokenExpiration.Has(err) {
			server.log.Debug("account activation",
				zap.String("token", activationToken),
				zap.Error(err),
			)
			http.Redirect(w, r, server.config.ExternalAddress+"activate?expired=true", http.StatusTemporaryRedirect)
			return
		}

		if console.ErrEmailUsed.Has(err) {
			server.log.Debug("account activation",
				zap.String("token", activationToken),
				zap.Error(err),
			)
			http.Redirect(w, r, server.config.ExternalAddress+"login?activated=false", http.StatusTemporaryRedirect)
			return
		}

		if console.Error.Has(err) {
			server.log.Error("activation: failed to activate account with a valid token",
				zap.Error(err))
			server.serveError(w, http.StatusInternalServerError)
			return
		}

		server.log.Error(
			"activation: failed to activate account with a valid token and unknown error type. BUG: missed error type check",
			zap.Error(err))
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	ip, err := web.GetRequestIP(r)
	if err != nil {
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	tokenInfo, err := server.service.GenerateSessionToken(ctx, user.ID, user.Email, ip, r.UserAgent())
	if err != nil {
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	server.cookieAuth.SetTokenCookie(w, *tokenInfo)

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
	rootObject[consoleql.ActivationPath] = "activation?token="
	rootObject[consoleql.PasswordRecoveryPath] = "password-recovery?token="
	rootObject[consoleql.CancelPasswordRecoveryPath] = "cancel-password-recovery?token="
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

// serveError serves a static error page.
func (server *Server) serveError(w http.ResponseWriter, status int) {
	w.WriteHeader(status)

	template, err := server.loadErrorTemplate()
	if err != nil {
		server.log.Error("unable to load error template", zap.Error(err))
		return
	}

	data := struct{ StatusCode int }{StatusCode: status}
	err = template.Execute(w, data)
	if err != nil {
		server.log.Error("cannot parse error template", zap.Error(err))
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

//go:embed error_fallback.html
var errorTemplateFallback string

// loadTemplates is used to initialize the error page template.
func (server *Server) loadErrorTemplate() (_ *template.Template, err error) {
	if server.errorTemplate == nil || server.config.Watch {
		server.errorTemplate, err = template.ParseFiles(filepath.Join(server.config.StaticDir, "static", "errors", "error.html"))
		if err != nil {
			server.log.Error("failed to load error.html template, falling back to error_fallback.html", zap.Error(err))
			server.errorTemplate, err = template.New("").Parse(errorTemplateFallback)
			if err != nil {
				return nil, Error.Wrap(err)
			}
		}
	}

	return server.errorTemplate, nil
}

// NewUserIDRateLimiter constructs a RateLimiter that limits based on user ID.
func NewUserIDRateLimiter(config web.RateLimiterConfig, log *zap.Logger) *web.RateLimiter {
	return web.NewRateLimiter(config, log, func(r *http.Request) (string, error) {
		user, err := console.GetUser(r.Context())
		if err != nil {
			return "", err
		}
		return user.ID.String(), nil
	})
}

// responseWriterStatusCode is a wrapper of an http.ResponseWriter to track the
// response status code for having access to it after calling
// http.ResponseWriter.WriteHeader.
type responseWriterStatusCode struct {
	http.ResponseWriter
	code int
}

func (w *responseWriterStatusCode) WriteHeader(code int) {
	w.code = code
	w.ResponseWriter.WriteHeader(code)
}

// newTraceRequestMiddleware returns middleware for tracing each request to a
// registered endpoint through Monkit.
//
// It also log in INFO level each request.
func newTraceRequestMiddleware(log *zap.Logger, root *mux.Router) mux.MiddlewareFunc {
	log = log.Named("trace-request-middleware")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			begin := time.Now()
			ctx := r.Context()
			respWCode := responseWriterStatusCode{ResponseWriter: w, code: 0}
			defer func() {
				// Preallocate the maximum fields that we are going to use for avoiding
				// reallocations
				fields := make([]zapcore.Field, 0, 6)
				fields = append(fields,
					zap.String("method", r.Method),
					zap.String("URI", r.RequestURI),
					zap.String("IP", getClientIP(r)),
					zap.Int("response-code", respWCode.code),
					zap.Duration("elapse", time.Since(begin)),
				)

				span := monkit.SpanFromCtx(ctx)
				if span != nil {
					fields = append(fields, zap.Int64("trace-id", span.Trace().Id()))
				}

				log.Info("client HTTP request", fields...)
			}()

			match := mux.RouteMatch{}
			root.Match(r, &match)

			pathTpl, err := match.Route.GetPathTemplate()
			if err != nil {
				log.Warn("error when getting the route template path",
					zap.Error(err), zap.String("request-uri", r.RequestURI),
				)
				next.ServeHTTP(&respWCode, r)
				return
			}

			// Limit the values accepted as an HTTP method for avoiding to create an
			// unbounded amount of metrics.
			boundMethod := r.Method
			switch r.Method {
			case http.MethodDelete:
			case http.MethodGet:
			case http.MethodHead:
			case http.MethodOptions:
			case http.MethodPatch:
			case http.MethodPost:
			case http.MethodPut:
			default:
				boundMethod = "INVALID"
			}

			stop := mon.TaskNamed("visit_task", monkit.NewSeriesTag("path", pathTpl), monkit.NewSeriesTag("method", boundMethod))(&ctx)
			r = r.WithContext(ctx)

			defer func() {
				var err error
				if respWCode.code >= http.StatusBadRequest {
					err = fmt.Errorf("%d", respWCode.code)
				}

				stop(&err)
				// Count the status codes returned by each endpoint.
				mon.Event("visit_event_by_code",
					monkit.NewSeriesTag("path", pathTpl),
					monkit.NewSeriesTag("method", boundMethod),
					monkit.NewSeriesTag("code", strconv.Itoa(respWCode.code)),
				)
			}()

			// Count the requests to each endpoint.
			mon.Event("visit_event", monkit.NewSeriesTag("path", pathTpl), monkit.NewSeriesTag("method", boundMethod))

			next.ServeHTTP(&respWCode, r)
		})
	}
}
