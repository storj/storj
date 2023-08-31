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
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/sync/errgroup"

	"storj.io/common/errs2"
	"storj.io/common/http/requestid"
	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/abtesting"
	"storj.io/storj/satellite/analytics"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi"
	"storj.io/storj/satellite/console/consoleweb/consolewebauth"
	"storj.io/storj/satellite/mailservice"
	"storj.io/storj/satellite/oidc"
	"storj.io/storj/satellite/payments/paymentsconfig"
)

const (
	contentType = "Content-Type"

	applicationJSON = "application/json"
)

var (
	// Error is satellite console error type.
	Error = errs.Class("consoleweb")

	mon = monkit.Package()
)

// Config contains configuration for console web server.
type Config struct {
	Address             string `help:"server address of the http api gateway and frontend app" devDefault:"127.0.0.1:0" releaseDefault:":10100"`
	FrontendAddress     string `help:"server address of the front-end app" devDefault:"127.0.0.1:0" releaseDefault:":10200"`
	ExternalAddress     string `help:"external endpoint of the satellite if hosted" default:""`
	FrontendEnable      bool   `help:"feature flag to toggle whether console back-end server should also serve front-end endpoints" default:"true"`
	BackendReverseProxy string `help:"the target URL of console back-end reverse proxy for local development when running a UI server" default:""`

	StaticDir string `help:"path to static resources" default:""`
	Watch     bool   `help:"whether to load templates on each request" default:"false" devDefault:"true"`

	AuthToken        string `help:"auth token needed for access to registration token creation endpoint" default:"" testDefault:"very-secret-token"`
	AuthTokenSecret  string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`
	AuthCookieDomain string `help:"optional domain for cookies to use" default:""`

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
	GatewayCredentialsRequestURL    string     `help:"url link for gateway credentials requests" default:"https://auth.storjsatelliteshare.io" devDefault:"http://localhost:8000"`
	IsBetaSatellite                 bool       `help:"indicates if satellite is in beta" default:"false"`
	BetaSatelliteFeedbackURL        string     `help:"url link for for beta satellite feedback" default:""`
	BetaSatelliteSupportURL         string     `help:"url link for for beta satellite support" default:""`
	DocumentationURL                string     `help:"url link to documentation" default:"https://docs.storj.io/"`
	CouponCodeBillingUIEnabled      bool       `help:"indicates if user is allowed to add coupon codes to account from billing" default:"false"`
	CouponCodeSignupUIEnabled       bool       `help:"indicates if user is allowed to add coupon codes to account from signup" default:"false"`
	FileBrowserFlowDisabled         bool       `help:"indicates if file browser flow is disabled" default:"false"`
	CSPEnabled                      bool       `help:"indicates if Content Security Policy is enabled" devDefault:"false" releaseDefault:"true"`
	LinksharingURL                  string     `help:"url link for linksharing requests within the application" default:"https://link.storjsatelliteshare.io" devDefault:"http://localhost:8001"`
	PublicLinksharingURL            string     `help:"url link for linksharing requests for external sharing" default:"https://link.storjshare.io" devDefault:"http://localhost:8001"`
	PathwayOverviewEnabled          bool       `help:"indicates if the overview onboarding step should render with pathways" default:"true"`
	AllProjectsDashboard            bool       `help:"indicates if all projects dashboard should be used" default:"true"`
	LimitsAreaEnabled               bool       `help:"indicates whether limit card section of the UI is enabled" default:"true"`
	GeneratedAPIEnabled             bool       `help:"indicates if generated console api should be used" default:"false"`
	OptionalSignupSuccessURL        string     `help:"optional url to external registration success page" default:""`
	HomepageURL                     string     `help:"url link to storj.io homepage" default:"https://www.storj.io"`
	NativeTokenPaymentsEnabled      bool       `help:"indicates if storj native token payments system is enabled" default:"false"`
	PricingPackagesEnabled          bool       `help:"whether to allow purchasing pricing packages" default:"false" devDefault:"true"`
	NewUploadModalEnabled           bool       `help:"whether to show new upload modal" default:"false"`
	GalleryViewEnabled              bool       `help:"whether to show new gallery view" default:"true"`
	UseVuetifyProject               bool       `help:"whether to use vuetify POC project" default:"false"`
	VuetifyHost                     string     `help:"the subdomain the vuetify POC project should be hosted on" default:""`

	OauthCodeExpiry         time.Duration `help:"how long oauth authorization codes are issued for" default:"10m"`
	OauthAccessTokenExpiry  time.Duration `help:"how long oauth access tokens are issued for" default:"24h"`
	OauthRefreshTokenExpiry time.Duration `help:"how long oauth refresh tokens are issued for" default:"720h"`

	BodySizeLimit memory.Size `help:"The maximum body size allowed to be received by the API" default:"100.00 KB"`

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
	router            *mux.Router
	cookieAuth        *consolewebauth.CookieAuth
	ipRateLimiter     *web.RateLimiter
	userIDRateLimiter *web.RateLimiter
	nodeURL           storj.NodeURL

	stripePublicKey                 string
	neededTokenPaymentConfirmations int

	packagePlans paymentsconfig.PackagePlans

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
func NewServer(logger *zap.Logger, config Config, service *console.Service, oidcService *oidc.Service, mailService *mailservice.Service, analytics *analytics.Service, abTesting *abtesting.Service, accountFreezeService *console.AccountFreezeService, listener net.Listener, stripePublicKey string, neededTokenPaymentConfirmations int, nodeURL storj.NodeURL, packagePlans paymentsconfig.PackagePlans) *Server {
	server := Server{
		log:                             logger,
		config:                          config,
		listener:                        listener,
		service:                         service,
		mailService:                     mailService,
		analytics:                       analytics,
		abTesting:                       abTesting,
		stripePublicKey:                 stripePublicKey,
		neededTokenPaymentConfirmations: neededTokenPaymentConfirmations,
		ipRateLimiter:                   web.NewIPRateLimiter(config.RateLimit, logger),
		userIDRateLimiter:               NewUserIDRateLimiter(config.RateLimit, logger),
		nodeURL:                         nodeURL,
		packagePlans:                    packagePlans,
	}

	logger.Debug("Starting Satellite Console server.", zap.Stringer("Address", server.listener.Addr()))

	server.cookieAuth = consolewebauth.NewCookieAuth(consolewebauth.CookieSettings{
		Name: "_tokenKey",
		Path: "/",
	}, server.config.AuthCookieDomain)

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
	server.router = router
	// N.B. This middleware has to be the first one because it has to be called
	// the earliest in the HTTP chain.
	router.Use(newTraceRequestMiddleware(logger, router))

	router.Use(requestid.AddToContext)

	// limit body size
	router.Use(newBodyLimiterMiddleware(logger.Named("body-limiter-middleware"), config.BodySizeLimit))

	if server.config.GeneratedAPIEnabled {
		consoleapi.NewProjectManagement(logger, mon, server.service, router, &apiAuth{&server})
		consoleapi.NewAPIKeyManagement(logger, mon, server.service, router, &apiAuth{&server})
		consoleapi.NewUserManagement(logger, mon, server.service, router, &apiAuth{&server})
	}

	router.Handle("/api/v0/config", server.withCORS(http.HandlerFunc(server.frontendConfigHandler)))

	router.HandleFunc("/registrationToken/", server.createRegistrationTokenHandler)
	router.HandleFunc("/robots.txt", server.seoHandler)

	projectsController := consoleapi.NewProjects(logger, service)
	projectsRouter := router.PathPrefix("/api/v0/projects").Subrouter()
	projectsRouter.Use(server.withCORS)
	projectsRouter.Use(server.withAuth)
	projectsRouter.Handle("", http.HandlerFunc(projectsController.GetUserProjects)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("", http.HandlerFunc(projectsController.CreateProject)).Methods(http.MethodPost, http.MethodOptions)
	projectsRouter.Handle("/paged", http.HandlerFunc(projectsController.GetPagedProjects)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/{id}", http.HandlerFunc(projectsController.UpdateProject)).Methods(http.MethodPatch, http.MethodOptions)
	projectsRouter.Handle("/{id}/members", http.HandlerFunc(projectsController.DeleteMembersAndInvitations)).Methods(http.MethodDelete, http.MethodOptions)
	projectsRouter.Handle("/{id}/salt", http.HandlerFunc(projectsController.GetSalt)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/{id}/members", http.HandlerFunc(projectsController.GetMembersAndInvitations)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/{id}/invite", http.HandlerFunc(projectsController.InviteUsers)).Methods(http.MethodPost, http.MethodOptions)
	projectsRouter.Handle("/{id}/invite-link", http.HandlerFunc(projectsController.GetInviteLink)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/invitations", http.HandlerFunc(projectsController.GetUserInvitations)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/invitations/{id}/respond", http.HandlerFunc(projectsController.RespondToInvitation)).Methods(http.MethodPost, http.MethodOptions)

	usageLimitsController := consoleapi.NewUsageLimits(logger, service)
	projectsRouter.Handle("/{id}/usage-limits", http.HandlerFunc(usageLimitsController.ProjectUsageLimits)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/usage-limits", http.HandlerFunc(usageLimitsController.TotalUsageLimits)).Methods(http.MethodGet, http.MethodOptions)
	projectsRouter.Handle("/{id}/daily-usage", http.HandlerFunc(usageLimitsController.DailyUsage)).Methods(http.MethodGet, http.MethodOptions)

	authController := consoleapi.NewAuth(logger, service, accountFreezeService, mailService, server.cookieAuth, server.analytics, config.SatelliteName, server.config.ExternalAddress, config.LetUsKnowURL, config.TermsAndConditionsURL, config.ContactInfoURL, config.GeneralRequestURL)
	authRouter := router.PathPrefix("/api/v0/auth").Subrouter()
	authRouter.Use(server.withCORS)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.GetAccount))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/account", server.withAuth(http.HandlerFunc(authController.UpdateAccount))).Methods(http.MethodPatch, http.MethodOptions)
	authRouter.Handle("/account/change-email", server.withAuth(http.HandlerFunc(authController.ChangeEmail))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/account/change-password", server.withAuth(server.userIDRateLimiter.Limit(http.HandlerFunc(authController.ChangePassword)))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/account/freezestatus", server.withAuth(http.HandlerFunc(authController.GetFreezeStatus))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/account/settings", server.withAuth(http.HandlerFunc(authController.GetUserSettings))).Methods(http.MethodGet, http.MethodOptions)
	authRouter.Handle("/account/settings", server.withAuth(http.HandlerFunc(authController.SetUserSettings))).Methods(http.MethodPatch, http.MethodOptions)
	authRouter.Handle("/account/onboarding", server.withAuth(http.HandlerFunc(authController.SetOnboardingStatus))).Methods(http.MethodPatch, http.MethodOptions)
	authRouter.Handle("/account/delete", server.withAuth(http.HandlerFunc(authController.DeleteAccount))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/mfa/enable", server.withAuth(http.HandlerFunc(authController.EnableUserMFA))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/mfa/disable", server.withAuth(http.HandlerFunc(authController.DisableUserMFA))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/mfa/generate-secret-key", server.withAuth(http.HandlerFunc(authController.GenerateMFASecretKey))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/mfa/generate-recovery-codes", server.withAuth(http.HandlerFunc(authController.GenerateMFARecoveryCodes))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/logout", server.withAuth(http.HandlerFunc(authController.Logout))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/token", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Token))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/token-by-api-key", server.ipRateLimiter.Limit(http.HandlerFunc(authController.TokenByAPIKey))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/register", server.ipRateLimiter.Limit(http.HandlerFunc(authController.Register))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/forgot-password", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ForgotPassword))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/resend-email/{email}", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResendEmail))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/reset-password", server.ipRateLimiter.Limit(http.HandlerFunc(authController.ResetPassword))).Methods(http.MethodPost, http.MethodOptions)
	authRouter.Handle("/refresh-session", server.withAuth(http.HandlerFunc(authController.RefreshSession))).Methods(http.MethodPost, http.MethodOptions)

	if config.ABTesting.Enabled {
		abController := consoleapi.NewABTesting(logger, abTesting)
		abRouter := router.PathPrefix("/api/v0/ab").Subrouter()
		abRouter.Use(server.withCORS)
		abRouter.Use(server.withAuth)
		abRouter.Handle("/values", http.HandlerFunc(abController.GetABValues)).Methods(http.MethodGet, http.MethodOptions)
		abRouter.Handle("/hit/{action}", http.HandlerFunc(abController.SendHit)).Methods(http.MethodPost, http.MethodOptions)
	}

	paymentController := consoleapi.NewPayments(logger, service, accountFreezeService, packagePlans)
	paymentsRouter := router.PathPrefix("/api/v0/payments").Subrouter()
	paymentsRouter.Use(server.withCORS)
	paymentsRouter.Use(server.withAuth)
	paymentsRouter.Handle("/cards", server.userIDRateLimiter.Limit(http.HandlerFunc(paymentController.AddCreditCard))).Methods(http.MethodPost, http.MethodOptions)
	paymentsRouter.HandleFunc("/cards", paymentController.MakeCreditCardDefault).Methods(http.MethodPatch, http.MethodOptions)
	paymentsRouter.HandleFunc("/cards", paymentController.ListCreditCards).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/cards/{cardId}", paymentController.RemoveCreditCard).Methods(http.MethodDelete, http.MethodOptions)
	paymentsRouter.HandleFunc("/account/charges", paymentController.ProjectsCharges).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/account/balance", paymentController.AccountBalance).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/account", paymentController.SetupAccount).Methods(http.MethodPost, http.MethodOptions)
	paymentsRouter.HandleFunc("/wallet", paymentController.GetWallet).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/wallet", paymentController.ClaimWallet).Methods(http.MethodPost, http.MethodOptions)
	paymentsRouter.HandleFunc("/wallet/payments", paymentController.WalletPayments).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/wallet/payments-with-confirmations", paymentController.WalletPaymentsWithConfirmations).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/billing-history", paymentController.BillingHistory).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/invoice-history", paymentController.InvoiceHistory).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.Handle("/coupon/apply", server.userIDRateLimiter.Limit(http.HandlerFunc(paymentController.ApplyCouponCode))).Methods(http.MethodPatch, http.MethodOptions)
	paymentsRouter.HandleFunc("/coupon", paymentController.GetCoupon).Methods(http.MethodGet, http.MethodOptions)
	paymentsRouter.HandleFunc("/pricing", paymentController.GetProjectUsagePriceModel).Methods(http.MethodGet, http.MethodOptions)
	if config.PricingPackagesEnabled {
		paymentsRouter.HandleFunc("/purchase-package", paymentController.PurchasePackage).Methods(http.MethodPost, http.MethodOptions)
		paymentsRouter.HandleFunc("/package-available", paymentController.PackageAvailable).Methods(http.MethodGet, http.MethodOptions)
	}

	bucketsController := consoleapi.NewBuckets(logger, service)
	bucketsRouter := router.PathPrefix("/api/v0/buckets").Subrouter()
	bucketsRouter.Use(server.withCORS)
	bucketsRouter.Use(server.withAuth)
	bucketsRouter.HandleFunc("/bucket-names", bucketsController.AllBucketNames).Methods(http.MethodGet, http.MethodOptions)
	bucketsRouter.HandleFunc("/usage-totals", bucketsController.GetBucketTotals).Methods(http.MethodGet, http.MethodOptions)

	apiKeysController := consoleapi.NewAPIKeys(logger, service)
	apiKeysRouter := router.PathPrefix("/api/v0/api-keys").Subrouter()
	apiKeysRouter.Use(server.withCORS)
	apiKeysRouter.Use(server.withAuth)
	apiKeysRouter.HandleFunc("/create/{projectID}", apiKeysController.CreateAPIKey).Methods(http.MethodPost, http.MethodOptions)
	apiKeysRouter.HandleFunc("/list-paged", apiKeysController.GetProjectAPIKeys).Methods(http.MethodGet, http.MethodOptions)
	apiKeysRouter.HandleFunc("/delete-by-name", apiKeysController.DeleteByNameAndProjectID).Methods(http.MethodDelete, http.MethodOptions)
	apiKeysRouter.HandleFunc("/delete-by-ids", apiKeysController.DeleteByIDs).Methods(http.MethodDelete, http.MethodOptions)
	apiKeysRouter.HandleFunc("/api-key-names", apiKeysController.GetAllAPIKeyNames).Methods(http.MethodGet, http.MethodOptions)

	analyticsController := consoleapi.NewAnalytics(logger, service, server.analytics)
	analyticsRouter := router.PathPrefix("/api/v0/analytics").Subrouter()
	analyticsRouter.Use(server.withCORS)
	analyticsRouter.Use(server.withAuth)
	analyticsRouter.HandleFunc("/event", analyticsController.EventTriggered).Methods(http.MethodPost, http.MethodOptions)
	analyticsRouter.HandleFunc("/page", analyticsController.PageEventTriggered).Methods(http.MethodPost, http.MethodOptions)

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

	router.HandleFunc("/invited", server.handleInvited)
	router.HandleFunc("/activation", server.accountActivationHandler)
	router.HandleFunc("/cancel-password-recovery", server.cancelPasswordRecoveryHandler)

	if server.config.StaticDir != "" && server.config.FrontendEnable {
		fs := http.FileServer(http.Dir(server.config.StaticDir))
		router.PathPrefix("/static/").Handler(server.withCORS(server.brotliMiddleware(http.StripPrefix("/static", fs))))

		if server.config.UseVuetifyProject {
			if server.config.VuetifyHost != "" {
				router.Host(server.config.VuetifyHost).Handler(server.withCORS(http.HandlerFunc(server.vuetifyAppHandler)))
			} else {
				// if not using a custom subdomain for vuetify, use a path prefix on the same domain as the production app
				router.PathPrefix("/vuetifypoc").Handler(server.withCORS(http.HandlerFunc(server.vuetifyAppHandler)))
			}
		}
		router.PathPrefix("/").Handler(server.withCORS(http.HandlerFunc(server.appHandler)))
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

// NewFrontendServer creates new instance of console front-end server.
// NB: The return type is currently consoleweb.Server, but it does not contain all the dependencies.
// It should only be used with RunFrontEnd and Close. We plan on moving this to its own type, but
// right now since we have a feature flag to allow the backend server to continue serving the frontend, it
// makes it easier if they are the same type.
func NewFrontendServer(logger *zap.Logger, config Config, listener net.Listener, nodeURL storj.NodeURL, stripePublicKey string) (server *Server, err error) {
	server = &Server{
		log:             logger,
		config:          config,
		listener:        listener,
		nodeURL:         nodeURL,
		stripePublicKey: stripePublicKey,
	}

	logger.Debug("Starting Satellite UI server.", zap.Stringer("Address", server.listener.Addr()))

	router := mux.NewRouter()

	// N.B. This middleware has to be the first one because it has to be called
	// the earliest in the HTTP chain.
	router.Use(newTraceRequestMiddleware(logger, router))

	// in local development, proxy certain requests to the console back-end server
	if config.BackendReverseProxy != "" {
		target, err := url.Parse(config.BackendReverseProxy)
		if err != nil {
			return nil, Error.Wrap(err)
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		logger.Debug("Reverse proxy targeting", zap.String("address", config.BackendReverseProxy))

		router.PathPrefix("/api").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		}))
		router.PathPrefix("/oauth").Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		}))
		router.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
		router.HandleFunc("/invited", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
		router.HandleFunc("/activation", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
		router.HandleFunc("/cancel-password-recovery", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
		router.HandleFunc("/registrationToken/", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
		router.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
			proxy.ServeHTTP(w, r)
		})
	}

	fs := http.FileServer(http.Dir(server.config.StaticDir))

	router.HandleFunc("/robots.txt", server.seoHandler)
	router.PathPrefix("/static/").Handler(server.brotliMiddleware(http.StripPrefix("/static", fs)))
	router.HandleFunc("/config", server.frontendConfigHandler)
	if server.config.UseVuetifyProject {
		router.PathPrefix("/vuetifypoc").Handler(http.HandlerFunc(server.vuetifyAppHandler))
	}
	router.PathPrefix("/").Handler(http.HandlerFunc(server.appHandler))
	server.server = http.Server{
		Handler:        server.withRequest(router),
		MaxHeaderBytes: ContentLengthLimit.Int(),
	}
	return server, nil
}

// RunFrontend starts the server that runs the webapp.
func (server *Server) RunFrontend(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	ctx, cancel := context.WithCancel(ctx)
	var group errgroup.Group
	group.Go(func() error {
		<-ctx.Done()
		return server.server.Shutdown(context.Background())
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

// setAppHeaders sets the necessary headers for requests to the app.
func (server *Server) setAppHeaders(w http.ResponseWriter, r *http.Request) {
	header := w.Header()

	if server.config.CSPEnabled {
		cspValues := []string{
			"default-src 'self'",
			"script-src 'sha256-wAqYV6m2PHGd1WDyFBnZmSoyfCK0jxFAns0vGbdiWUA=' 'self' *.stripe.com https://www.google.com/recaptcha/ https://www.gstatic.com/recaptcha/ https://hcaptcha.com *.hcaptcha.com",
			"connect-src 'self' *.tardigradeshare.io *.storjshare.io *.storjapi.io https://hcaptcha.com *.hcaptcha.com " + server.config.GatewayCredentialsRequestURL,
			"frame-ancestors " + server.config.FrameAncestors,
			"frame-src 'self' *.stripe.com https://www.google.com/recaptcha/ https://recaptcha.google.com/recaptcha/ https://hcaptcha.com *.hcaptcha.com",
			"img-src 'self' data: blob: *.tardigradeshare.io *.storjshare.io *.storjsatelliteshare.io",
			// Those are hashes of charts custom tooltip inline styles. They have to be updated if styles are updated.
			"style-src 'unsafe-hashes' 'sha256-7mY2NKmZ4PuyjGUa4FYC5u36SxXdoUM/zxrlr3BEToo=' 'sha256-PRTMwLUW5ce9tdiUrVCGKqj6wPeuOwGogb1pmyuXhgI=' 'sha256-kwpt3lQZ21rs4cld7/uEm9qI5yAbjYzx+9FGm/XmwNU=' 'sha256-Qf4xqtNKtDLwxce6HLtD5Y6BWpOeR7TnDpNSo+Bhb3s=' 'self' https://hcaptcha.com *.hcaptcha.com",
			"media-src 'self' blob: *.tardigradeshare.io *.storjshare.io *.storjsatelliteshare.io",
		}

		header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "same-origin") // Only expose the referring url when navigating around the satellite itself.
}

// appHandler is web app http handler function.
func (server *Server) appHandler(w http.ResponseWriter, r *http.Request) {
	server.setAppHeaders(w, r)

	path := filepath.Join(server.config.StaticDir, "dist", "index.html")
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			server.log.Error("index.html was not generated. run 'npm run build' in the "+server.config.StaticDir+" directory", zap.Error(err))
		} else {
			server.log.Error("error loading index.html", zap.String("path", path), zap.Error(err))
		}
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

// vuetifyAppHandler is web app http handler function.
func (server *Server) vuetifyAppHandler(w http.ResponseWriter, r *http.Request) {
	server.setAppHeaders(w, r)

	path := filepath.Join(server.config.StaticDir, "dist_vuetify_poc", "index-vuetify.html")
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			server.log.Error("index-vuetify.html was not generated. run 'npm run build-vuetify' in the "+server.config.StaticDir+" directory", zap.Error(err))
		} else {
			server.log.Error("error loading index-vuetify.html", zap.String("path", path), zap.Error(err))
		}
		return
	}

	defer func() {
		if err := file.Close(); err != nil {
			server.log.Error("error closing index-vuetify.html", zap.String("path", path), zap.Error(err))
		}
	}()

	info, err := file.Stat()
	if err != nil {
		server.log.Error("failed to retrieve index-vuetify.html file info", zap.Error(err))
		return
	}

	http.ServeContent(w, r, path, info.ModTime(), file)
}

// withCORS handles setting CORS-related headers on an http request.
func (server *Server) withCORS(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", strings.Trim(server.config.ExternalAddress, "/"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", "*, Authorization")

		if r.Method == http.MethodOptions {
			match := &mux.RouteMatch{}
			if server.router.Match(r, match) {
				methods, err := match.Route.GetMethods()
				if err == nil && len(methods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
				}
			}
			return
		}

		handler.ServeHTTP(w, r)
	})
}

// withAuth performs initial authorization before every request.
func (server *Server) withAuth(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		ctx := r.Context()

		defer mon.Task()(&ctx)(&err)

		defer func() {
			if err != nil {
				web.ServeJSONError(ctx, server.log, w, http.StatusUnauthorized, console.ErrUnauthorized.Wrap(err))
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
		PublicLinksharingURL:            server.config.PublicLinksharingURL,
		PathwayOverviewEnabled:          server.config.PathwayOverviewEnabled,
		DefaultPaidStorageLimit:         server.config.UsageLimits.Storage.Paid,
		DefaultPaidBandwidthLimit:       server.config.UsageLimits.Bandwidth.Paid,
		Captcha:                         server.config.Captcha,
		AllProjectsDashboard:            server.config.AllProjectsDashboard,
		LimitsAreaEnabled:               server.config.LimitsAreaEnabled,
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
		NewUploadModalEnabled:           server.config.NewUploadModalEnabled,
		GalleryViewEnabled:              server.config.GalleryViewEnabled,
		NeededTransactionConfirmations:  server.neededTokenPaymentConfirmations,
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

func (server *Server) handleInvited(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	token := r.URL.Query().Get("invite")
	if token == "" {
		server.serveError(w, http.StatusBadRequest)
		return
	}

	loginLink := server.config.ExternalAddress + "login"

	invite, err := server.service.GetInviteByToken(ctx, token)
	if err != nil {
		server.log.Error("handleInvited: error checking invitation", zap.Error(err))

		if console.ErrProjectInviteInvalid.Has(err) {
			http.Redirect(w, r, loginLink+"?invite_invalid=true", http.StatusTemporaryRedirect)
			return
		}
		server.serveError(w, http.StatusInternalServerError)
		return
	}

	user, _, err := server.service.GetUserByEmailWithUnverified(ctx, invite.Email)
	if err != nil && !console.ErrEmailNotFound.Has(err) {
		server.log.Error("error getting invitation recipient", zap.Error(err))
		server.serveError(w, http.StatusInternalServerError)
		return
	}
	if user != nil {
		http.Redirect(w, r, loginLink+"?email="+url.QueryEscape(user.Email), http.StatusTemporaryRedirect)
		return
	}

	params := url.Values{"email": {strings.ToLower(invite.Email)}}

	if invite.InviterID != nil {
		inviter, err := server.service.GetUser(ctx, *invite.InviterID)
		if err != nil {
			server.log.Error("error getting invitation sender", zap.Error(err))
			server.serveError(w, http.StatusInternalServerError)
			return
		}
		name := inviter.ShortName
		if name == "" {
			name = inviter.FullName
		}
		params.Add("inviter", name)
		params.Add("inviter_email", inviter.Email)
	}

	proj, err := server.service.GetProjectNoAuth(ctx, invite.ProjectID)
	if err != nil {
		server.log.Error("error getting invitation project", zap.Error(err))
		server.serveError(w, http.StatusInternalServerError)
		return
	}
	params.Add("project", proj.Name)

	http.Redirect(w, r, server.config.ExternalAddress+"signup?"+params.Encode(), http.StatusTemporaryRedirect)
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

// newBodyLimiterMiddleware returns a middleware that places a length limit on each request's body.
func newBodyLimiterMiddleware(log *zap.Logger, limit memory.Size) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit.Int64() {
				web.ServeJSONError(r.Context(), log, w, http.StatusRequestEntityTooLarge, errs.New("Request body is too large"))
				return
			}

			r.Body = http.MaxBytesReader(w, r.Body, limit.Int64())
			next.ServeHTTP(w, r)
		})
	}
}
