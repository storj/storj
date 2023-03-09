// AUTOGENERATED BY private/apigen
// DO NOT EDIT.

package example

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/api"
)

const dateLayout = "2006-01-02T15:04:05.999Z"

var ErrTestapiAPI = errs.Class("example testapi api")

type TestAPIService interface {
	GenTestAPI(ctx context.Context, path string, id uuid.UUID, date time.Time, request struct{ Content string }) (*struct {
		ID        uuid.UUID
		Date      time.Time
		PathParam string
		Body      string
	}, api.HTTPError)
}

// TestAPIHandler is an api handler that exposes all testapi related functionality.
type TestAPIHandler struct {
	log     *zap.Logger
	mon     *monkit.Scope
	service TestAPIService
	auth    api.Auth
}

func NewTestAPI(log *zap.Logger, mon *monkit.Scope, service TestAPIService, router *mux.Router, auth api.Auth) *TestAPIHandler {
	handler := &TestAPIHandler{
		log:     log,
		mon:     mon,
		service: service,
		auth:    auth,
	}

	testapiRouter := router.PathPrefix("/api/v0/testapi").Subrouter()
	testapiRouter.HandleFunc("/{path}", handler.handleGenTestAPI).Methods("POST")

	return handler
}

func (h *TestAPIHandler) handleGenTestAPI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer h.mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam := r.URL.Query().Get("id")
	if idParam == "" {
		api.ServeError(h.log, w, http.StatusBadRequest, errs.New("parameter 'id' can't be empty"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		api.ServeError(h.log, w, http.StatusBadRequest, err)
		return
	}

	dateParam := r.URL.Query().Get("date")
	if dateParam == "" {
		api.ServeError(h.log, w, http.StatusBadRequest, errs.New("parameter 'date' can't be empty"))
		return
	}

	date, err := time.Parse(dateLayout, dateParam)
	if err != nil {
		api.ServeError(h.log, w, http.StatusBadRequest, err)
		return
	}

	path, ok := mux.Vars(r)["path"]
	if !ok {
		api.ServeError(h.log, w, http.StatusBadRequest, errs.New("missing path route param"))
		return
	}

	payload := struct{ Content string }{}
	if err = json.NewDecoder(r.Body).Decode(&payload); err != nil {
		api.ServeError(h.log, w, http.StatusBadRequest, err)
		return
	}

	ctx, err = h.auth.IsAuthenticated(ctx, r, true, true)
	if err != nil {
		h.auth.RemoveAuthCookie(w)
		api.ServeError(h.log, w, http.StatusUnauthorized, err)
		return
	}

	retVal, httpErr := h.service.GenTestAPI(ctx, path, id, date, payload)
	if httpErr.Err != nil {
		api.ServeError(h.log, w, httpErr.Status, httpErr.Err)
		return
	}

	err = json.NewEncoder(w).Encode(retVal)
	if err != nil {
		h.log.Debug("failed to write json GenTestAPI response", zap.Error(ErrTestapiAPI.Wrap(err)))
	}
}
