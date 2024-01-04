// AUTOGENERATED BY private/apigen
// DO NOT EDIT.

package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/private/api"
)

var ErrSettingsAPI = errs.Class("admin settings api")
var ErrPlacementsAPI = errs.Class("admin placements api")
var ErrUsersAPI = errs.Class("admin users api")

type SettingsService interface {
	GetSettings(ctx context.Context) (*Settings, api.HTTPError)
}

type PlacementManagementService interface {
	GetPlacements(ctx context.Context) ([]PlacementInfo, api.HTTPError)
}

type UserManagementService interface {
	GetUserByEmail(ctx context.Context, email string) (*User, api.HTTPError)
}

// SettingsHandler is an api handler that implements all Settings API endpoints functionality.
type SettingsHandler struct {
	log     *zap.Logger
	mon     *monkit.Scope
	service SettingsService
}

// PlacementManagementHandler is an api handler that implements all PlacementManagement API endpoints functionality.
type PlacementManagementHandler struct {
	log     *zap.Logger
	mon     *monkit.Scope
	service PlacementManagementService
}

// UserManagementHandler is an api handler that implements all UserManagement API endpoints functionality.
type UserManagementHandler struct {
	log     *zap.Logger
	mon     *monkit.Scope
	service UserManagementService
	auth    *Authorizer
}

func NewSettings(log *zap.Logger, mon *monkit.Scope, service SettingsService, router *mux.Router) *SettingsHandler {
	handler := &SettingsHandler{
		log:     log,
		mon:     mon,
		service: service,
	}

	settingsRouter := router.PathPrefix("/back-office/api/v1/settings").Subrouter()
	settingsRouter.HandleFunc("/", handler.handleGetSettings).Methods("GET")

	return handler
}

func NewPlacementManagement(log *zap.Logger, mon *monkit.Scope, service PlacementManagementService, router *mux.Router) *PlacementManagementHandler {
	handler := &PlacementManagementHandler{
		log:     log,
		mon:     mon,
		service: service,
	}

	placementsRouter := router.PathPrefix("/back-office/api/v1/placements").Subrouter()
	placementsRouter.HandleFunc("/", handler.handleGetPlacements).Methods("GET")

	return handler
}

func NewUserManagement(log *zap.Logger, mon *monkit.Scope, service UserManagementService, router *mux.Router, auth *Authorizer) *UserManagementHandler {
	handler := &UserManagementHandler{
		log:     log,
		mon:     mon,
		service: service,
		auth:    auth,
	}

	usersRouter := router.PathPrefix("/back-office/api/v1/users").Subrouter()
	usersRouter.HandleFunc("/{email}", handler.handleGetUserByEmail).Methods("GET")

	return handler
}

func (h *SettingsHandler) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer h.mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	retVal, httpErr := h.service.GetSettings(ctx)
	if httpErr.Err != nil {
		api.ServeError(h.log, w, httpErr.Status, httpErr.Err)
		return
	}

	err = json.NewEncoder(w).Encode(retVal)
	if err != nil {
		h.log.Debug("failed to write json GetSettings response", zap.Error(ErrSettingsAPI.Wrap(err)))
	}
}

func (h *PlacementManagementHandler) handleGetPlacements(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer h.mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	retVal, httpErr := h.service.GetPlacements(ctx)
	if httpErr.Err != nil {
		api.ServeError(h.log, w, httpErr.Status, httpErr.Err)
		return
	}

	err = json.NewEncoder(w).Encode(retVal)
	if err != nil {
		h.log.Debug("failed to write json GetPlacements response", zap.Error(ErrPlacementsAPI.Wrap(err)))
	}
}

func (h *UserManagementHandler) handleGetUserByEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer h.mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	email, ok := mux.Vars(r)["email"]
	if !ok {
		api.ServeError(h.log, w, http.StatusBadRequest, errs.New("missing email route param"))
		return
	}

	if h.auth.IsRejected(w, r, 1) {
		return
	}

	retVal, httpErr := h.service.GetUserByEmail(ctx, email)
	if httpErr.Err != nil {
		api.ServeError(h.log, w, httpErr.Status, httpErr.Err)
		return
	}

	err = json.NewEncoder(w).Encode(retVal)
	if err != nil {
		h.log.Debug("failed to write json GetUserByEmail response", zap.Error(ErrUsersAPI.Wrap(err)))
	}
}
