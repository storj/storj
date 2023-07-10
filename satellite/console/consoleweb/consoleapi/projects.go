// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
)

// Projects is an api controller that exposes projects related functionality.
type Projects struct {
	log     *zap.Logger
	service *console.Service
}

// NewProjects is a constructor for api analytics controller.
func NewProjects(log *zap.Logger, service *console.Service) *Projects {
	return &Projects{
		log:     log,
		service: service,
	}
}

// GetSalt returns the project's salt.
func (p *Projects) GetSalt(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
	}

	salt, err := p.service.GetSalt(ctx, id)
	if err != nil {
		p.serveJSONError(w, http.StatusUnauthorized, err)
		return
	}

	b64SaltString := base64.StdEncoding.EncodeToString(salt)

	err = json.NewEncoder(w).Encode(b64SaltString)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// InviteUsers sends invites to a given project(id) to the given users (emails).
func (p *Projects) InviteUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
	}

	var data struct {
		Emails []string `json:"emails"`
	}

	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	for i, email := range data.Emails {
		data.Emails[i] = strings.TrimSpace(email)
	}

	_, err = p.service.InviteProjectMembers(ctx, id, data.Emails)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// GetInviteLink returns a link to an invitation given project ID and invitee's email.
func (p *Projects) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)
	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing email query param"))
		return
	}

	link, err := p.service.GetInviteLink(ctx, id, email)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}

	err = json.NewEncoder(w).Encode(link)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// GetUserInvitations returns the user's pending project member invitations.
func (p *Projects) GetUserInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	invites, err := p.service.GetUserProjectInvitations(ctx)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
		return
	}

	type jsonInvite struct {
		ProjectID          uuid.UUID `json:"projectID"`
		ProjectName        string    `json:"projectName"`
		ProjectDescription string    `json:"projectDescription"`
		InviterEmail       string    `json:"inviterEmail"`
		CreatedAt          time.Time `json:"createdAt"`
	}

	response := make([]jsonInvite, 0)

	for _, invite := range invites {
		proj, err := p.service.GetProjectNoAuth(ctx, invite.ProjectID)
		if err != nil {
			p.serveJSONError(w, http.StatusInternalServerError, err)
			return
		}

		respInvite := jsonInvite{
			ProjectID:          proj.PublicID,
			ProjectName:        proj.Name,
			ProjectDescription: proj.Description,
			CreatedAt:          invite.CreatedAt,
		}

		if invite.InviterID != nil {
			inviter, err := p.service.GetUser(ctx, *invite.InviterID)
			if err != nil {
				p.serveJSONError(w, http.StatusInternalServerError, err)
				return
			}
			respInvite.InviterEmail = inviter.Email
		}

		response = append(response, respInvite)
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		p.serveJSONError(w, http.StatusInternalServerError, err)
	}
}

// RespondToInvitation handles accepting or declining a user's project member invitation.
func (p *Projects) RespondToInvitation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
	}

	var payload struct {
		Response console.ProjectInvitationResponse `json:"response"`
	}

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(w, http.StatusBadRequest, err)
		return
	}

	err = p.service.RespondToProjectInvitation(ctx, id, payload.Response)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case console.ErrAlreadyMember.Has(err):
			status = http.StatusConflict
		case console.ErrProjectInviteInvalid.Has(err):
			status = http.StatusNotFound
		case console.ErrValidation.Has(err):
			status = http.StatusBadRequest
		}
		p.serveJSONError(w, status, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (p *Projects) serveJSONError(w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(p.log, w, status, err)
}
