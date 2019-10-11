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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"storj.io/storj/internal/post"

	"github.com/prometheus/common/log"

	"github.com/gorilla/mux"
	"github.com/graphql-go/graphql"
	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

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
	StripeKey       string `help:"stripe api key" default:""`

	// TODO: remove after Vanguard release
	AuthToken       string `help:"auth token needed for access to registration token creation endpoint" default:""`
	AuthTokenSecret string `help:"secret used to sign auth tokens" releaseDefault:"" devDefault:"my-suppa-secret-key"`

	PasswordCost int `internal:"true" help:"password hashing cost (0=automatic)" default:"0"`

	SatelliteName         string `help:"used to display at web satellite console" default:"Storj"`
	SatelliteOperator     string `help:"name of organization which set up satellite" default:"Storj Labs" `
	LetUsKnowURL          string `help:"url link to let us know page" default:"https://storjlabs.atlassian.net/servicedesk/customer/portals"`
	ContactInfoURL        string `help:"url link to contacts page" default:"https://forum.storj.io"`
	TermsAndConditionsURL string `help:"url link to terms and conditions page" default:"https://storj.io/storage-sla/"`
	SEO                   string `help:"used to communicate with web crawlers and other web robots" default:"User-agent: *\nDisallow: \nDisallow: /cgi-bin/"`
}

// Server represents console web server
//
// architecture: Endpoint
type Server struct {
	log *zap.Logger

	config      Config
	service     *console.Service
	mailService *mailservice.Service

	listener net.Listener
	server   http.Server

	schema    graphql.Schema
	templates struct {
		index         *template.Template
		pageNotFound  *template.Template
		usageReport   *template.Template
		resetPassword *template.Template
		success       *template.Template
		activated     *template.Template
	}
}

type RootObject struct {
	Origin                     string
	ActivationPath             string
	PasswordRecoveryPath       string
	CancelPasswordRecoveryPath string
	SignInPath                 string
	LetUsKnowURL               string
	ContactInfoURL             string
	TermsAndConditionsURL      string
}

// NewServer creates new instance of console server
func NewServer(logger *zap.Logger, config Config, service *console.Service, mailService *mailservice.Service, listener net.Listener) *Server {
	server := Server{
		log:         logger,
		config:      config,
		listener:    listener,
		service:     service,
		mailService: mailService,
	}

	logger.Sugar().Debugf("Starting Satellite UI on %s...", server.listener.Addr().String())

	if server.config.ExternalAddress != "" {
		if !strings.HasSuffix(server.config.ExternalAddress, "/") {
			server.config.ExternalAddress += "/"
		}
	} else {
		server.config.ExternalAddress = "http://" + server.listener.Addr().String() + "/"
	}

	fs := http.FileServer(http.Dir(server.config.StaticDir))

	router := mux.NewRouter()
	router.Handle("/api/graphql/v0", http.HandlerFunc(server.grapqlHandler))

	usersRouter := router.PathPrefix("/users").Subrouter()
	usersRouter.Use(server.authMiddlewareHandler)

	usersRouter.Handle("/token/", http.HandlerFunc(server.tokenRequestHandler)).Methods("POST")
	usersRouter.Handle("/", http.HandlerFunc(server.createNewUserRequestHandler)).Methods("POST")
	usersRouter.Handle("/", http.HandlerFunc(server.deleteAccountRequestHandler)).Methods("DELETE")
	usersRouter.Handle("/change-password/", http.HandlerFunc(server.changeAccountPasswordRequestHandler)).Methods("POST")
	usersRouter.Handle("/{id}/resend-email/", http.HandlerFunc(server.resendEmailRequestHandler)).Methods("GET")
	usersRouter.Handle("/{email}/forgot-password/", http.HandlerFunc(server.forgotPasswordRequestHandler)).Methods("GET")

	if server.config.StaticDir != "" {
		router.Handle("/activation/", http.HandlerFunc(server.accountActivationHandler))
		router.Handle("/password-recovery/", http.HandlerFunc(server.passwordRecoveryHandler))
		router.Handle("/cancel-password-recovery/", http.HandlerFunc(server.cancelPasswordRecoveryHandler))
		router.Handle("/registrationToken/", http.HandlerFunc(server.createRegistrationTokenHandler))
		router.Handle("/usage-report/", http.HandlerFunc(server.bucketUsageReportHandler))
		router.Handle("/static/", server.gzipHandler(http.StripPrefix("/static", fs)))
		router.Handle("/robots.txt", http.HandlerFunc(server.seoHandler))

		router.Handle("/", http.HandlerFunc(server.appHandler))
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
		"script-src 'self' *.stripe.com cdn.segment.com",
		"frame-src 'self' *.stripe.com",
		"img-src 'self' data:",
	}

	header.Set(contentType, "text/html; charset=UTF-8")
	header.Set("Content-Security-Policy", strings.Join(cspValues, "; "))
	header.Set("X-Content-Type-Options", "nosniff")

	if server.templates.index == nil || server.templates.index.Execute(w, nil) != nil {
		server.log.Error("satellite/console/server: index template could not be executed")
		server.serveError(w, r, http.StatusNotFound)
		return
	}
}
// authMiddlewareHandler performs initial authorization before every request
func (server *Server) authMiddlewareHandler(handler http.Handler) http.Handler {
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

		rootObject := RootObject{
			Origin:                     server.config.ExternalAddress,
			ActivationPath:             "activation/?token=",
			PasswordRecoveryPath:       "password-recovery/?token=",
			CancelPasswordRecoveryPath: "cancel-password-recovery/?token=",
			SignInPath:                 "login",
			LetUsKnowURL:               server.config.LetUsKnowURL,
			ContactInfoURL:             server.config.ContactInfoURL,
			TermsAndConditionsURL:      server.config.TermsAndConditionsURL,
		}

		ctx = context.WithValue(ctx, "rootObject", rootObject)

		handler.ServeHTTP(w, r.Clone(ctx))
	})
}

// tokenRequestHandler authenticates User by credentials and returns auth token
func (server *Server) tokenRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	type tokenRequestModel struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	var tokenRequest tokenRequestModel
	err = json.NewDecoder(r.Body).Decode(&tokenRequest)
	if err != nil {
		server.serveJsonError(w, 400, err)
	}

	token, err := server.service.Token(ctx, tokenRequest.Email, tokenRequest.Password)
	if err != nil {
		server.serveJsonError(w, 404, err)
		return
	}

	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		server.serveJsonError(w, 500, err)
		server.log.Debug("Error serializing response: " + err.Error())
	}
}

// changeAccountPasswordRequestHandler updates password for a given user
func (server *Server) changeAccountPasswordRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	_, err = console.GetAuth(ctx)
	if err != nil {
		server.serveJsonError(w, 401, err)
		return
	}

	type ChangePasswordRequestModel struct {
		Password    string `json:"password"`
		NewPassword string `json:"newPassword"`
	}

	var passwordChange ChangePasswordRequestModel
	err = json.NewDecoder(r.Body).Decode(&passwordChange)
	if err != nil {
		server.serveJsonError(w, 400, err)
		return
	}

	err = server.service.ChangePassword(ctx, passwordChange.Password, passwordChange.NewPassword)
	if err != nil {
		server.serveJsonError(w, 404, err)
		return
	}
}

// createNewUserRequestHandler gets password hash value and creates new inactive User
func (server *Server) createNewUserRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	type createUserRequestModel struct {
		console.CreateUser
		Secret         string `json:"secret"`
		ReferrerUserID string `json:"referrerUserId"`
	}

	var model createUserRequestModel
	err = json.NewDecoder(r.Body).Decode(&model)
	if err != nil {
		server.serveJsonError(w, 400, err)
		return
	}

	secret, err := console.RegistrationSecretFromBase64(model.Secret)
	if err != nil {
		log.Error("register: failed to create account",
			zap.Error(err))
		log.Debug("register: ", zap.String("rawSecret", model.Secret))
		server.serveJsonError(w, 400, err)

		return
	}

	user, err := server.service.CreateUser(ctx, model.CreateUser, secret, model.ReferrerUserID)
	if err != nil {
		log.Error("register: failed to create account",
			zap.Error(err))
		log.Debug("register: ", zap.String("rawSecret", model.Secret))
		server.serveJsonError(w, 400, err)

		return
	}

	token, err := server.service.GenerateActivationToken(ctx, user.ID, user.Email)
	if err != nil {
		log.Error("register: failed to generate activation token",
			zap.Stringer("id", user.ID),
			zap.String("email", user.Email),
			zap.Error(err))
		server.serveJsonError(w, 400, err)

		return
	}

	rootObject, ok := ctx.Value("rootObject").(RootObject)
	if !ok {
		server.log.Error("root object is not set")
		return
	}
	link := rootObject.Origin + rootObject.ActivationPath + token
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	server.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.AccountActivationEmail{
			Origin:         rootObject.Origin,
			ActivationLink: link,
		},
	)

	err = json.NewEncoder(w).Encode(user)
	if err != nil {
		server.serveJsonError(w, 500, err)
		server.log.Debug("Error serializing response: " + err.Error())
	}
}

// deleteAccountRequestHandler deletes User
func (server *Server) deleteAccountRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	auth, err := console.GetAuth(ctx)
	if err != nil {
		server.serveJsonError(w, 404, err)
		w.WriteHeader(401)
		return
	}

	type deleteAccountRequestModel struct {
		Password string `json:"password"`
	}

	var password deleteAccountRequestModel
	err = json.NewDecoder(r.Body).Decode(&password)
	if err != nil {
		server.serveJsonError(w, 404, err)
		w.WriteHeader(400)
		return
	}

	err = server.service.DeleteAccount(ctx, password.Password)
	if err != nil {
		server.serveJsonError(w, 404, err)
		return
	}

	err = json.NewEncoder(w).Encode(auth.User)
	if err != nil {
		server.serveJsonError(w, 404, err)
		w.WriteHeader(500)
		server.log.Debug("Error serializing response: " + err.Error())
	}
}

// resendEmailRequestHandler resend activation email for given email
func (server *Server) resendEmailRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)
	params := mux.Vars(r)
	val, ok := params["id"]
	if !ok {
		err = errs.New("id expected")

		server.serveJsonError(w, 400, err)
		return
	}

	userID, err := uuid.Parse(val)
	if err != nil {
		server.serveJsonError(w, 400, err)
		return
	}

	user, err := server.service.GetUser(ctx, *userID)
	if err != nil {
		server.serveJsonError(w, 404, err)
		return
	}
	token, err := server.service.GenerateActivationToken(ctx, user.ID, user.Email)

	rootObject, ok := ctx.Value("rootObject").(RootObject)
	if !ok {
		server.log.Error("root object is not set")
		return
	}
	link := rootObject.Origin + rootObject.ActivationPath + token
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	contactInfoURL := rootObject.ContactInfoURL
	termsAndConditionsURL := rootObject.TermsAndConditionsURL

	server.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.AccountActivationEmail{
			Origin:                rootObject.Origin,
			ActivationLink:        link,
			TermsAndConditionsURL: termsAndConditionsURL,
			ContactInfoURL:        contactInfoURL,
		},
	)
}

// forgotPasswordRequestHandler creates reset password token and send user email
func (server *Server) forgotPasswordRequestHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	params := mux.Vars(r)
	email, ok := params["email"]
	if !ok {
		err = errs.New("email expected")

		server.serveJsonError(w, 400, err)
		return
	}

	user, err := server.service.GetUserByEmail(ctx, email)
	if err != nil {
		server.serveJsonError(w, 404, err)
		return
	}

	recoveryToken, err := server.service.GeneratePasswordRecoveryToken(ctx, user.ID)
	if err != nil {
		server.serveJsonError(w, 500, errs.New("failed to generate password recovery token"))
	}

	rootObject := ctx.Value("rootObject").(RootObject)
	passwordRecoveryLink := rootObject.Origin + rootObject.PasswordRecoveryPath + recoveryToken
	cancelPasswordRecoveryLink := rootObject.Origin + rootObject.CancelPasswordRecoveryPath + recoveryToken
	userName := user.ShortName
	if user.ShortName == "" {
		userName = user.FullName
	}

	contactInfoURL := rootObject.ContactInfoURL
	letUsKnowURL := rootObject.LetUsKnowURL
	termsAndConditionsURL := rootObject.TermsAndConditionsURL

	server.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: userName}},
		&consoleql.ForgotPasswordEmail{
			Origin:                     rootObject.Origin,
			ResetLink:                  passwordRecoveryLink,
			CancelPasswordRecoveryLink: cancelPasswordRecoveryLink,
			UserName:                   userName,
			LetUsKnowURL:               letUsKnowURL,
			TermsAndConditionsURL:      termsAndConditionsURL,
			ContactInfoURL:             contactInfoURL,
		},
	)

}

// bucketUsageReportHandler generate bucket usage report page for project
func (server *Server) bucketUsageReportHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	tokenCookie, err := r.Cookie("_tokenKey")
	if err != nil {
		server.serveError(w, r, http.StatusUnauthorized)
		return
	}

	auth, err := server.service.Authorize(auth.WithAPIKey(ctx, []byte(tokenCookie.Value)))
	if err != nil {
		server.serveError(w, r, http.StatusUnauthorized)
		return
	}

	ctx = console.WithAuth(ctx, auth)

	// parse query params
	projectID, err := uuid.Parse(r.URL.Query().Get("projectID"))
	if err != nil {
		server.serveError(w, r, http.StatusBadRequest)
		return
	}
	sinceStamp, err := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	if err != nil {
		server.serveError(w, r, http.StatusBadRequest)
		return
	}
	beforeStamp, err := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	if err != nil {
		server.serveError(w, r, http.StatusBadRequest)
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
		server.serveError(w, r, http.StatusInternalServerError)
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
		server.serveError(w, r, http.StatusNotFound)
		return
	}

	if err = server.templates.activated.Execute(w, nil); err != nil {
		server.log.Error("satellite/console/server: account activated template could not be executed", zap.Error(err))
		server.serveError(w, r, http.StatusNotFound)
		return
	}
}

func (server *Server) passwordRecoveryHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	defer mon.Task()(&ctx)(nil)

	recoveryToken := r.URL.Query().Get("token")
	if len(recoveryToken) == 0 {
		server.serveError(w, r, http.StatusNotFound)
		return
	}

	switch r.Method {
	case http.MethodPost:
		err := r.ParseForm()
		if err != nil {
			server.serveError(w, r, http.StatusNotFound)
			return
		}

		password := r.FormValue("password")
		passwordRepeat := r.FormValue("passwordRepeat")
		if strings.Compare(password, passwordRepeat) != 0 {
			server.serveError(w, r, http.StatusNotFound)
			return
		}

		err = server.service.ResetPassword(ctx, recoveryToken, password)
		if err != nil {
			server.serveError(w, r, http.StatusNotFound)
			return
		}

		if err := server.templates.success.Execute(w, nil); err != nil {
			server.log.Error("satellite/console/server: success reset password template could not be executed", zap.Error(err))
			server.serveError(w, r, http.StatusNotFound)
			return
		}
	case http.MethodGet:
		if err := server.templates.resetPassword.Execute(w, nil); err != nil {
			server.log.Error("satellite/console/server: reset password template could not be executed", zap.Error(err))
			server.serveError(w, r, http.StatusNotFound)
			return
		}
	default:
		server.serveError(w, r, http.StatusNotFound)
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

func (server *Server) serveError(w http.ResponseWriter, r *http.Request, status int) {
	// TODO: show different error pages depend on status
	// F.e. switch(status)
	//      case http.StatusNotFound: server.executeTemplate(w, r, notFound, nil)
	//      case http.StatusInternalServerError: server.executeTemplate(w, r, internalError, nil)
	w.WriteHeader(status)

	if err := server.templates.pageNotFound.Execute(w, nil); err != nil {
		server.log.Error("error occurred in console/server", zap.Error(err))
	}
}

func (server *Server) serveJsonError(w http.ResponseWriter, status int, err error) {
	w.WriteHeader(status)
	if err == nil {
		return
	}

	server.log.Error("error occurred in console/server", zap.Error(err))

	err = json.NewEncoder(w).Encode(err.Error())
	if err != nil {
		server.log.Error("error while serializing error response")
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

// gzipHandler is used to gzip static content to minify resources if browser support such decoding
func (server *Server) gzipHandler(fn http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isGzipSupported := strings.Contains(r.Header.Get("Accept-Encoding"), "gzip")
		extension := filepath.Ext(r.RequestURI)
		// we have gzipped only fonts, js and css bundles
		formats := map[string]bool{
			".js":  true,
			".ttf": true,
			".css": true,
		}
		isNeededFormatToGzip := formats[extension]

		// because we have some static content outside of console frontend app.
		// for example: 404 page, account activation, passsrowd reset, etc.
		// TODO: find better solution, its a temporary fix
		isFromStaticDir := strings.Contains(r.URL.Path, "/static/dist/")

		w.Header().Set(contentType, mime.TypeByExtension(extension))
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// in case if old browser doesn't support gzip decoding or if file extension is not recommended to gzip
		// just return original file
		if !isGzipSupported || !isNeededFormatToGzip || !isFromStaticDir {
			fn.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")

		// updating request URL
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

	server.templates.pageNotFound, err = template.ParseFiles(path.Join(server.config.StaticDir, "static", "errors", "404.html"))
	if err != nil {
		return Error.Wrap(err)
	}

	return nil
}
