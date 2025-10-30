// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/private/web"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/console/consoleweb/consoleapi/utils"
)

// Projects is an api controller that exposes projects related functionality.
type Projects struct {
	log     *zap.Logger
	service *console.Service
}

// ProjectMembersPage contains information about a page of project members and invitations.
type ProjectMembersPage struct {
	Members        []Member     `json:"projectMembers"`
	Invitations    []Invitation `json:"projectInvitations"`
	TotalCount     int          `json:"totalCount"`
	Offset         int          `json:"offset"`
	Limit          int          `json:"limit"`
	Order          int          `json:"order"`
	OrderDirection int          `json:"orderDirection"`
	Search         string       `json:"search"`
	CurrentPage    int          `json:"currentPage"`
	PageCount      int          `json:"pageCount"`
}

// Member is a project member in a ProjectMembersPage.
type Member struct {
	ID        uuid.UUID                 `json:"id"`
	FullName  string                    `json:"fullName"`
	ShortName string                    `json:"shortName"`
	Email     string                    `json:"email"`
	Role      console.ProjectMemberRole `json:"role"`
	JoinedAt  time.Time                 `json:"joinedAt"`
}

// Invitation is a project invitation in a ProjectMembersPage.
type Invitation struct {
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
	Expired   bool      `json:"expired"`
}

// NewProjects is a constructor for api analytics controller.
func NewProjects(log *zap.Logger, service *console.Service) *Projects {
	return &Projects{
		log:     log,
		service: service,
	}
}

// GetUserProjects returns the user's projects.
func (p *Projects) GetUserProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	projects, err := p.service.GetUsersProjects(ctx)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	response := make([]console.ProjectInfo, 0)
	for _, project := range projects {
		response = append(response, p.service.GetMinimalProject(&project))
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// UpdateProject handles updating projects.
func (p *Projects) UpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var payload console.UpsertProjectInfo

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	_, err = p.service.UpdateProject(ctx, id, payload)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrInvalidProjectLimit.Has(err) || console.ErrValidation.Has(err) {
			p.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// DeleteProject handles deleting projects.
func (p *Projects) DeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var data AccountActionData
	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if data.Step < console.DeleteProjectInit || data.Step > console.DeleteProjectStep {
		p.serveJSONError(ctx, w, http.StatusBadRequest, console.ErrValidation.New("step value is out of range"))
		return
	}

	if data.Step > console.DeleteProjectInit && data.Step != console.DeleteProjectStep && data.Data == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, console.ErrValidation.New("data value can't be empty"))
		return
	}

	resp, err := p.service.DeleteProject(ctx, id, data.Step, data.Data)
	if err != nil {
		if resp != nil {
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				p.log.Error("could not encode project deletion response", zap.Error(ErrAuthAPI.Wrap(err)))
			}
			return
		}
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// UpdateUserSpecifiedLimits is a method for updating project user specified limits.
func (p *Projects) UpdateUserSpecifiedLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var payload console.UpdateLimitsInfo

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	err = p.service.UpdateUserSpecifiedLimits(ctx, id, payload)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrInvalidProjectLimit.Has(err) || console.ErrValidation.Has(err) {
			p.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// RequestLimitIncrease handles requesting limit increase for projects.
func (p *Projects) RequestLimitIncrease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var payload console.LimitRequestInfo

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	if payload.LimitType == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing limit type"))
		return
	}
	if payload.DesiredLimit == 0 {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing desired limit"))
		return
	}
	if payload.CurrentLimit == 0 {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing current limit"))
		return
	}

	err = p.service.RequestLimitIncrease(ctx, id, payload)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// CreateProject handles creating projects.
func (p *Projects) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var payload console.UpsertProjectInfo

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	project, err := p.service.CreateProject(ctx, payload)
	if err != nil {
		if console.ErrBotUser.Has(err) {
			p.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}

		if console.ErrValidation.Has(err) {
			p.serveJSONError(ctx, w, http.StatusBadRequest, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(p.service.GetMinimalProject(project))
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// GetMembersAndInvitations returns the project's members and invitees.
func (p *Projects) GetMembersAndInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	publicID, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	project, err := p.service.GetProject(ctx, publicID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	if limitStr == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing limit query param"))
		return
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid limit parameter: %s", limitStr))
		return
	}

	search := r.URL.Query().Get("search")

	pageStr := r.URL.Query().Get("page")
	if pageStr == "" {
		pageStr = "1"
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid page parameter: %s", pageStr))
		return
	}

	orderStr := r.URL.Query().Get("order")
	if orderStr == "" {
		orderStr = "1"
	}
	order, err := strconv.Atoi(orderStr)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid order parameter: %s", orderStr))
		return
	}

	orderDirStr := r.URL.Query().Get("order-direction")
	if orderDirStr == "" {
		orderDirStr = "1"
	}
	orderDir, err := strconv.Atoi(orderDirStr)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid order-direction parameter: %s", orderDirStr))
		return
	}

	var memberPage ProjectMembersPage
	membersAndInvitations, err := p.service.GetProjectMembersAndInvitations(ctx, project.ID, console.ProjectMembersCursor{
		Search:         search,
		Limit:          uint(limit),
		Page:           uint(page),
		Order:          console.ProjectMemberOrder(order),
		OrderDirection: console.OrderDirection(orderDir),
	})
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}
	memberPage.Search = membersAndInvitations.Search
	memberPage.Limit = int(membersAndInvitations.Limit)
	memberPage.Order = int(membersAndInvitations.Order)
	memberPage.OrderDirection = int(membersAndInvitations.OrderDirection)
	memberPage.Offset = int(membersAndInvitations.Offset)
	memberPage.PageCount = int(membersAndInvitations.PageCount)
	memberPage.CurrentPage = int(membersAndInvitations.CurrentPage)
	memberPage.TotalCount = int(membersAndInvitations.TotalCount)
	memberPage.Members = []Member{}
	memberPage.Invitations = []Invitation{}

	for _, m := range membersAndInvitations.ProjectMembers {
		user, err := p.service.GetUser(ctx, m.MemberID)
		if err != nil {
			p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
			return
		}
		member := Member{
			ID:        user.ID,
			FullName:  user.FullName,
			ShortName: user.ShortName,
			Email:     user.Email,
			Role:      m.Role,
			JoinedAt:  m.CreatedAt,
		}
		memberPage.Members = append(memberPage.Members, member)
	}
	for _, inv := range membersAndInvitations.ProjectInvitations {
		invitee := Invitation{
			Email:     inv.Email,
			CreatedAt: inv.CreatedAt,
			Expired:   p.service.IsProjectInvitationExpired(&inv),
		}
		memberPage.Invitations = append(memberPage.Invitations, invitee)
	}
	err = json.NewEncoder(w).Encode(memberPage)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// UpdateMemberRole updates project member role.
func (p *Projects) UpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	projectIDParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	publicID, err := uuid.FromString(projectIDParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	memberIDParam, ok := mux.Vars(r)["memberID"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing member id route param"))
		return
	}

	memberID, err := uuid.FromString(memberIDParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	newRoleBytes, err := io.ReadAll(r.Body)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	newRoleInt, err := strconv.Atoi(string(newRoleBytes))
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var newRole console.ProjectMemberRole
	switch newRoleInt {
	case int(console.RoleAdmin), int(console.RoleMember):
		newRole = console.ProjectMemberRole(newRoleInt)
	default:
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("invalid role value"))
		return
	}

	updatedMember, err := p.service.UpdateProjectMemberRole(ctx, memberID, publicID, newRole)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if console.ErrForbidden.Has(err) {
			p.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}
		if console.ErrConflict.Has(err) {
			p.serveJSONError(ctx, w, http.StatusConflict, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	user, err := p.service.GetUser(ctx, updatedMember.MemberID)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	member := Member{
		ID:        user.ID,
		FullName:  user.FullName,
		ShortName: user.ShortName,
		Email:     user.Email,
		Role:      updatedMember.Role,
		JoinedAt:  updatedMember.CreatedAt,
	}

	err = json.NewEncoder(w).Encode(member)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// GetMember returns project member.
func (p *Projects) GetMember(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	projectIDParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	publicID, err := uuid.FromString(projectIDParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	memberIDParam, ok := mux.Vars(r)["memberID"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing member id route param"))
		return
	}

	memberID, err := uuid.FromString(memberIDParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	member, err := p.service.GetProjectMember(ctx, memberID, publicID)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	var returnedMember struct {
		ID              uuid.UUID                 `json:"id"`
		PublicProjectID uuid.UUID                 `json:"projectID"`
		Role            console.ProjectMemberRole `json:"role"`
		JoinedAt        time.Time                 `json:"joinedAt"`
	}

	returnedMember.ID = member.MemberID
	returnedMember.PublicProjectID = publicID
	returnedMember.Role = member.Role
	returnedMember.JoinedAt = member.CreatedAt

	err = json.NewEncoder(w).Encode(returnedMember)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
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
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	salt, err := p.service.GetSalt(ctx, id)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
		return
	}

	b64SaltString := base64.StdEncoding.EncodeToString(salt)

	err = json.NewEncoder(w).Encode(b64SaltString)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// GetEmissionImpact returns CO2 emission impact.
func (p *Projects) GetEmissionImpact(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	impact, err := p.service.GetEmissionImpact(ctx, id)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(impact)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// GetConfig returns config specific to a project.
func (p *Projects) GetConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	config, err := p.service.GetProjectConfig(ctx, id)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	err = json.NewEncoder(w).Encode(config)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// MigratePricing migrates classic project to use new storage tiers.
func (p *Projects) MigratePricing(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	w.Header().Set("Content-Type", "application/json")

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	err = p.service.MigrateProjectPricing(ctx, id)
	if err != nil {
		status := http.StatusInternalServerError

		switch {
		case console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err):
			status = http.StatusUnauthorized
		case console.ErrConflict.Has(err):
			status = http.StatusConflict
		case console.ErrForbidden.Has(err):
			status = http.StatusForbidden
		}
		p.serveJSONError(ctx, w, status, err)
	}
}

// InviteUser sends a project invitation to a user.
func (p *Projects) InviteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	email, ok := mux.Vars(r)["email"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing email route param"))
		return
	}
	email = strings.TrimSpace(email)

	isValidEmail := utils.ValidateEmail(email)
	if !isValidEmail {
		p.serveJSONError(ctx, w, http.StatusBadRequest, console.ErrValidation.Wrap(errs.New("Invalid email.")))
		return
	}

	_, err = p.service.InviteNewProjectMember(ctx, id, email)
	if err != nil {
		status := http.StatusInternalServerError
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			status = http.StatusUnauthorized
		} else if console.ErrNotPaidTier.Has(err) {
			status = http.StatusPaymentRequired
		}
		p.serveJSONError(ctx, w, status, err)
	}
}

// ReinviteUsers resends expired project invitations.
func (p *Projects) ReinviteUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	var data struct {
		Emails []string `json:"emails"`
	}

	err = json.NewDecoder(r.Body).Decode(&data)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	for i, email := range data.Emails {
		data.Emails[i] = strings.TrimSpace(email)
	}

	_, err = p.service.ReinviteProjectMembers(ctx, id, data.Emails)
	if err != nil {
		status := http.StatusInternalServerError
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			status = http.StatusUnauthorized
		} else if console.ErrNotPaidTier.Has(err) {
			status = http.StatusPaymentRequired
		}
		p.serveJSONError(ctx, w, status, err)
	}
}

// GetInviteLink returns a link to an invitation given project ID and invitee's email.
func (p *Projects) GetInviteLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	idParam, ok := mux.Vars(r)["id"]
	if !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}
	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	email := r.URL.Query().Get("email")
	if email == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing email query param"))
		return
	}

	link, err := p.service.GetInviteLink(ctx, id, email)
	if err != nil {
		if console.ErrUnauthorized.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		if console.ErrForbidden.Has(err) {
			p.serveJSONError(ctx, w, http.StatusForbidden, err)
			return
		}

		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}

	err = json.NewEncoder(w).Encode(link)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
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
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
		return
	}

	type jsonInvite struct {
		ProjectID          uuid.UUID `json:"projectID"`
		ProjectName        string    `json:"projectName"`
		ProjectDescription string    `json:"projectDescription"`
		InviterEmail       string    `json:"inviterEmail"`
		CreatedAt          time.Time `json:"createdAt"`
	}

	response := []jsonInvite{}

	for _, invite := range invites {
		proj, err := p.service.GetProjectNoAuth(ctx, invite.ProjectID)
		if err != nil {
			p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
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
				p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
				return
			}
			respInvite.InviterEmail = inviter.Email
		}

		response = append(response, respInvite)
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
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
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	var payload struct {
		Response console.ProjectInvitationResponse `json:"response"`
	}

	err = json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
		return
	}

	err = p.service.RespondToProjectInvitation(ctx, id, payload.Response)
	if err != nil {
		status := http.StatusInternalServerError
		switch {
		case console.ErrUnauthorized.Has(err):
			status = http.StatusUnauthorized
		case console.ErrAlreadyMember.Has(err):
			status = http.StatusConflict
		case console.ErrProjectInviteInvalid.Has(err):
			status = http.StatusNotFound
		case console.ErrValidation.Has(err):
			status = http.StatusBadRequest
		case console.ErrBotUser.Has(err):
			status = http.StatusForbidden
		}
		p.serveJSONError(ctx, w, status, err)
	}
}

// DeleteMembersAndInvitations deletes members and invitations from a project.
func (p *Projects) DeleteMembersAndInvitations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var err error
	defer mon.Task()(&ctx)(&err)

	var ok bool
	var idParam string

	if idParam, ok = mux.Vars(r)["id"]; !ok {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing project id route param"))
		return
	}

	id, err := uuid.FromString(idParam)
	if err != nil {
		p.serveJSONError(ctx, w, http.StatusBadRequest, err)
	}

	emailsStr := r.URL.Query().Get("emails")
	if emailsStr == "" {
		p.serveJSONError(ctx, w, http.StatusBadRequest, errs.New("missing emails parameter"))
		return
	}

	emails := strings.Split(emailsStr, ",")

	err = p.service.DeleteProjectMembersAndInvitations(ctx, id, emails)
	if err != nil {
		if console.ErrUnauthorized.Has(err) || console.ErrNoMembership.Has(err) {
			p.serveJSONError(ctx, w, http.StatusUnauthorized, err)
			return
		}
		p.serveJSONError(ctx, w, http.StatusInternalServerError, err)
	}
}

// serveJSONError writes JSON error to response output stream.
func (p *Projects) serveJSONError(ctx context.Context, w http.ResponseWriter, status int, err error) {
	web.ServeJSONError(ctx, p.log, w, status, err)
}
