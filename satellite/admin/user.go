// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/memory"
	"storj.io/common/storj"
	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
)

func (server *Server) addUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input console.CreateUser

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	user := console.CreateUser{
		Email:           input.Email,
		FullName:        input.FullName,
		Password:        input.Password,
		SignupPromoCode: input.SignupPromoCode,
	}

	err = user.IsValid(false)
	if err != nil {
		sendJSONError(w, "user data is not valid",
			err.Error(), http.StatusBadRequest)
		return
	}

	existingUser, err := server.db.Console().Users().GetByEmail(ctx, input.Email)
	if err != nil && !errors.Is(sql.ErrNoRows, err) {
		sendJSONError(w, "failed to check for user email",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if existingUser != nil {
		sendJSONError(w, fmt.Sprintf("user with email already exists %s", input.Email),
			"", http.StatusConflict)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 0)
	if err != nil {
		sendJSONError(w, "unable to save password hash",
			"", http.StatusInternalServerError)
		return
	}

	userID, err := uuid.New()
	if err != nil {
		sendJSONError(w, "unable to create UUID",
			"", http.StatusInternalServerError)
		return
	}

	newUser, err := server.db.Console().Users().Insert(ctx, &console.User{
		ID:                    userID,
		FullName:              user.FullName,
		ShortName:             user.ShortName,
		Email:                 user.Email,
		PasswordHash:          hash,
		ProjectLimit:          server.console.DefaultProjectLimit,
		ProjectStorageLimit:   server.console.UsageLimits.Storage.Free.Int64(),
		ProjectBandwidthLimit: server.console.UsageLimits.Bandwidth.Free.Int64(),
		SignupPromoCode:       user.SignupPromoCode,
	})
	if err != nil {
		sendJSONError(w, "failed to insert user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = server.payments.Setup(ctx, newUser.ID, newUser.Email, newUser.SignupPromoCode)
	if err != nil {
		sendJSONError(w, "failed to create payment account for user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	// Set User Status to be activated, as we manually created it
	newUser.Status = console.Active
	err = server.db.Console().Users().Update(ctx, userID, console.UpdateUserRequest{
		Status: &newUser.Status,
	})
	if err != nil {
		sendJSONError(w, "failed to activate user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(newUser)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}
	user.PasswordHash = nil

	projects, err := server.db.Console().Projects().GetOwn(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "failed to get user projects",
			err.Error(), http.StatusInternalServerError)
		return
	}

	type User struct {
		ID           uuid.UUID                 `json:"id"`
		FullName     string                    `json:"fullName"`
		Email        string                    `json:"email"`
		ProjectLimit int                       `json:"projectLimit"`
		Placement    storj.PlacementConstraint `json:"placement"`
		PaidTier     bool                      `json:"paidTier"`
	}
	type Project struct {
		ID          uuid.UUID `json:"id"`
		PublicID    uuid.UUID `json:"publicId"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		OwnerID     uuid.UUID `json:"ownerId"`
	}

	var output struct {
		User     User      `json:"user"`
		Projects []Project `json:"projects"`
	}

	output.User = User{
		ID:           user.ID,
		FullName:     user.FullName,
		Email:        user.Email,
		ProjectLimit: user.ProjectLimit,
		Placement:    user.DefaultPlacement,
		PaidTier:     user.PaidTier,
	}
	for _, p := range projects {
		output.Projects = append(output.Projects, Project{
			ID:          p.ID,
			PublicID:    p.PublicID,
			Name:        p.Name,
			Description: p.Description,
			OwnerID:     p.OwnerID,
		})
	}

	data, err := json.Marshal(output)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) usersPendingDeletion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	type User struct {
		ID       uuid.UUID `json:"id"`
		FullName string    `json:"fullName"`
		Email    string    `json:"email"`
	}

	query := r.URL.Query()

	limitParam := query.Get("limit")
	if limitParam == "" {
		sendJSONError(w, "Bad request", "parameter 'limit' can't be empty", http.StatusBadRequest)
		return
	}

	limit, err := strconv.ParseUint(limitParam, 10, 32)
	if err != nil {
		sendJSONError(w, "Bad request", err.Error(), http.StatusBadRequest)
		return
	}

	pageParam := query.Get("page")
	if pageParam == "" {
		sendJSONError(w, "Bad request", "parameter 'page' can't be empty", http.StatusBadRequest)
		return
	}

	page, err := strconv.ParseUint(pageParam, 10, 32)
	if err != nil {
		sendJSONError(w, "Bad request", err.Error(), http.StatusBadRequest)
		return
	}

	var sendingPage struct {
		Users       []User `json:"users"`
		PageCount   uint   `json:"pageCount"`
		CurrentPage uint   `json:"currentPage"`
		TotalCount  uint64 `json:"totalCount"`
		HasMore     bool   `json:"hasMore"`
	}
	usersPage, err := server.db.Console().Users().GetByStatus(
		ctx, console.PendingDeletion, console.UserCursor{
			Limit: uint(limit),
			Page:  uint(page),
		},
	)
	if err != nil {
		sendJSONError(w, "failed retrieving a usersPage of users", err.Error(), http.StatusInternalServerError)
		return
	}

	sendingPage.PageCount = usersPage.PageCount
	sendingPage.CurrentPage = usersPage.CurrentPage
	sendingPage.TotalCount = usersPage.TotalCount
	sendingPage.Users = make([]User, 0, len(usersPage.Users))

	if sendingPage.PageCount > sendingPage.CurrentPage {
		sendingPage.HasMore = true
	}

	for _, user := range usersPage.Users {
		invoices, err := server.payments.Invoices().ListFailed(ctx, &user.ID)
		if err != nil {
			sendJSONError(w, "getting invoices failed",
				err.Error(), http.StatusInternalServerError)
			return
		}
		if len(invoices) != 0 {
			sendingPage.TotalCount--
			continue
		}
		sendingPage.Users = append(sendingPage.Users, User{
			ID:       user.ID,
			FullName: user.FullName,
			Email:    user.Email,
		})
	}

	data, err := json.Marshal(sendingPage)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) userLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}
	user.PasswordHash = nil

	var limits struct {
		Storage   int64 `json:"storage"`
		Bandwidth int64 `json:"bandwidth"`
		Segment   int64 `json:"segment"`
	}

	limits.Storage = user.ProjectStorageLimit
	limits.Bandwidth = user.ProjectBandwidthLimit
	limits.Segment = user.ProjectSegmentLimit

	data, err := json.Marshal(limits)
	if err != nil {
		sendJSONError(w, "json encoding failed",
			err.Error(), http.StatusInternalServerError)
		return
	}

	sendJSONData(w, http.StatusOK, data)
}

func (server *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	type UserWithPaidTier struct {
		console.User
		PaidTierStr string `json:"paidTierStr"`
	}

	var input UserWithPaidTier

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	updateRequest := console.UpdateUserRequest{}

	if input.FullName != "" {
		updateRequest.FullName = &input.FullName
	}
	if input.ShortName != "" {
		shortNamePtr := &input.ShortName
		updateRequest.ShortName = &shortNamePtr
	}
	if input.Email != "" {
		existingUser, err := server.db.Console().Users().GetByEmail(ctx, input.Email)
		if err != nil && !errors.Is(sql.ErrNoRows, err) {
			sendJSONError(w, "failed to check for user email",
				err.Error(), http.StatusInternalServerError)
			return
		}
		if existingUser != nil {
			sendJSONError(w, fmt.Sprintf("user with email already exists %s", input.Email),
				"", http.StatusConflict)
			return
		}
		updateRequest.Email = &input.Email
	}
	if len(input.PasswordHash) > 0 {
		updateRequest.PasswordHash = input.PasswordHash
	}
	if input.ProjectLimit > 0 {
		updateRequest.ProjectLimit = &input.ProjectLimit
	}
	if input.ProjectStorageLimit > 0 {
		updateRequest.ProjectStorageLimit = &input.ProjectStorageLimit
	}
	if input.ProjectBandwidthLimit > 0 {
		updateRequest.ProjectBandwidthLimit = &input.ProjectBandwidthLimit
	}
	if input.ProjectSegmentLimit > 0 {
		updateRequest.ProjectSegmentLimit = &input.ProjectSegmentLimit
	}
	if input.PaidTierStr != "" {
		status, err := strconv.ParseBool(input.PaidTierStr)
		if err != nil {
			sendJSONError(w, "failed to parse paid tier status",
				err.Error(), http.StatusBadRequest)
			return
		}

		updateRequest.PaidTier = &status

		if status {
			now := server.nowFn()
			updateRequest.UpgradeTime = &now
		}
	}

	err = server.db.Console().Users().Update(ctx, user.ID, updateRequest)
	if err != nil {
		sendJSONError(w, "failed to update user",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) updateUsersUserAgent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	creationDatePlusMonth := user.CreatedAt.AddDate(0, 1, 0)
	if time.Now().After(creationDatePlusMonth) {
		sendJSONError(w, "this user was created more than a month ago",
			"we should update user agent only for recently created users", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		UserAgent string `json:"userAgent"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.UserAgent == "" {
		sendJSONError(w, "UserAgent was not provided",
			"", http.StatusBadRequest)
		return
	}

	newUserAgent := []byte(input.UserAgent)

	if bytes.Equal(user.UserAgent, newUserAgent) {
		sendJSONError(w, "new UserAgent is equal to existing users UserAgent",
			"", http.StatusBadRequest)
		return
	}

	err = server.db.Console().Users().UpdateUserAgent(ctx, user.ID, newUserAgent)
	if err != nil {
		sendJSONError(w, "failed to update user's user agent",
			err.Error(), http.StatusInternalServerError)
		return
	}

	projects, err := server.db.Console().Projects().GetOwn(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "failed to get users projects",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var errList errs.Group
	for _, project := range projects {
		if bytes.Equal(project.UserAgent, newUserAgent) {
			errList.Add(errs.New("projectID: %s. New UserAgent is equal to existing users UserAgent", project.ID))
			continue
		}

		err = server._updateProjectsUserAgent(ctx, project.ID, newUserAgent)
		if err != nil {
			errList.Add(errs.New("projectID: %s. Failed to update projects user agent: %s", project.ID, err))
		}
	}

	if errList.Err() != nil {
		sendJSONError(w, "failed to update projects user agent",
			errList.Err().Error(), http.StatusInternalServerError)
	}
}

// updateLimits updates user limits and all project limits for that user (future and existing).
func (server *Server) updateLimits(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		Storage   memory.Size `json:"storage"`
		Bandwidth memory.Size `json:"bandwidth"`
		Segment   int64       `json:"segment"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	newLimits := console.UsageLimits{
		Storage:   user.ProjectStorageLimit,
		Bandwidth: user.ProjectBandwidthLimit,
		Segment:   user.ProjectSegmentLimit,
	}

	if input.Storage > 0 {
		newLimits.Storage = input.Storage.Int64()
	}
	if input.Bandwidth > 0 {
		newLimits.Bandwidth = input.Bandwidth.Int64()
	}
	if input.Segment > 0 {
		newLimits.Segment = input.Segment
	}

	if newLimits.Storage == user.ProjectStorageLimit &&
		newLimits.Bandwidth == user.ProjectBandwidthLimit &&
		newLimits.Segment == user.ProjectSegmentLimit {
		sendJSONError(w, "no limits to update",
			"new values are equal to old ones", http.StatusBadRequest)
		return
	}

	err = server.db.Console().Users().UpdateUserProjectLimits(ctx, user.ID, newLimits)
	if err != nil {
		sendJSONError(w, "failed to update user limits",
			err.Error(), http.StatusInternalServerError)
		return
	}

	userProjects, err := server.db.Console().Projects().GetOwn(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "failed to get user's projects",
			err.Error(), http.StatusInternalServerError)
		return
	}

	for _, p := range userProjects {
		err = server.db.Console().Projects().UpdateUsageLimits(ctx, p.ID, newLimits)
		if err != nil {
			sendJSONError(w, "failed to update project limits",
				err.Error(), http.StatusInternalServerError)
		}
	}
}

func (server *Server) disableUserMFA(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	user.MFAEnabled = false
	user.MFASecretKey = ""
	mfaSecretKeyPtr := &user.MFASecretKey
	var mfaRecoveryCodes []string

	err = server.db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
		MFAEnabled:       &user.MFAEnabled,
		MFASecretKey:     &mfaSecretKeyPtr,
		MFARecoveryCodes: &mfaRecoveryCodes,
	})
	if err != nil {
		sendJSONError(w, "failed to disable mfa",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) billingFreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.BillingFreezeUser(ctx, u.ID)
	if err != nil {
		sendJSONError(w, "failed to billing freeze user",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) billingUnfreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.BillingUnfreezeUser(ctx, u.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusNotFound
		}
		sendJSONError(w, "failed to billing unfreeze user",
			err.Error(), status)
		return
	}
}

func (server *Server) billingUnWarnUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	if err = server.freezeAccounts.BillingUnWarnUser(ctx, u.ID); err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusNotFound
		}
		sendJSONError(w, "failed to billing unwarn user",
			err.Error(), status)
		return
	}
}

func (server *Server) violationFreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.ViolationFreezeUser(ctx, u.ID)
	if err != nil {
		sendJSONError(w, "failed to violation freeze user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	invoices, err := server.payments.Invoices().List(ctx, u.ID)
	if err != nil {
		server.log.Error("failed to get invoices for violation frozen user", zap.Error(err))
		return
	}

	for _, invoice := range invoices {
		if invoice.Status == payments.InvoiceStatusOpen {
			server.analytics.TrackViolationFrozenUnpaidInvoice(invoice.ID, u.ID, u.Email)
		}
	}
}

func (server *Server) violationUnfreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.ViolationUnfreezeUser(ctx, u.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusNotFound
		}
		sendJSONError(w, "failed to violation unfreeze user",
			err.Error(), status)
		return
	}
}

func (server *Server) legalFreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.LegalFreezeUser(ctx, u.ID)
	if err != nil {
		sendJSONError(w, "failed to legal freeze user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	invoices, err := server.payments.Invoices().List(ctx, u.ID)
	if err != nil {
		server.log.Error("failed to get invoices for legal frozen user", zap.Error(err))
		return
	}

	for _, invoice := range invoices {
		if invoice.Status == payments.InvoiceStatusOpen {
			server.analytics.TrackLegalHoldUnpaidInvoice(invoice.ID, u.ID, u.Email)
		}
	}
}

func (server *Server) legalUnfreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.LegalUnfreezeUser(ctx, u.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusNotFound
		}
		sendJSONError(w, "failed to legal unfreeze user",
			err.Error(), status)
		return
	}
}

func (server *Server) trialExpirationFreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.TrialExpirationFreezeUser(ctx, u.ID)
	if err != nil {
		sendJSONError(w, "failed to trial expiration freeze user",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) trialExpirationUnfreezeUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	u, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
				"", http.StatusNotFound)
			return
		}
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.freezeAccounts.TrialExpirationUnfreezeUser(ctx, u.ID, nil, nil)
	if err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusNotFound
		}
		sendJSONError(w, "failed to legal unfreeze user",
			err.Error(), status)
		return
	}
}

func (server *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure user has no own projects any longer
	projects, err := server.db.Console().Projects().GetOwn(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "unable to list projects",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(projects) > 0 {
		sendJSONError(w, "some projects still exist",
			fmt.Sprintf("%v", projects), http.StatusConflict)
		return
	}

	// Delete memberships in foreign projects
	members, err := server.db.Console().ProjectMembers().GetByMemberID(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "unable to search for user project memberships",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(members) > 0 {
		for _, project := range members {
			err := server.db.Console().ProjectMembers().Delete(ctx, user.ID, project.ProjectID)
			if err != nil {
				sendJSONError(w, "unable to delete user project membership",
					err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	// ensure no unpaid invoices exist.
	invoices, err := server.payments.Invoices().List(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "unable to list user invoices",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if len(invoices) > 0 {
		for _, invoice := range invoices {
			if invoice.Status == "draft" || invoice.Status == "open" {
				sendJSONError(w, "user has unpaid/pending invoices",
					"", http.StatusConflict)
				return
			}
		}
	}

	hasItems, err := server.payments.Invoices().CheckPendingItems(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "unable to list pending invoice items",
			err.Error(), http.StatusInternalServerError)
		return
	}
	if hasItems {
		sendJSONError(w, "user has pending invoice items",
			"", http.StatusConflict)
		return
	}

	emptyName := ""
	emptyNamePtr := &emptyName
	deactivatedEmail := fmt.Sprintf("deactivated+%s@storj.io", user.ID.String())
	status := console.Deleted

	err = server.db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{
		FullName:  &emptyName,
		ShortName: &emptyNamePtr,
		Email:     &deactivatedEmail,
		Status:    &status,
	})
	if err != nil {
		sendJSONError(w, "unable to delete user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	err = server.payments.CreditCards().RemoveAll(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "unable to delete credit card(s) from stripe account",
			err.Error(), http.StatusInternalServerError)
	}
}

func (server *Server) createGeofenceForAccount(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		Region string `json:"region"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	if input.Region == "" {
		sendJSONError(w, "region was not provided",
			"", http.StatusBadRequest)
		return
	}

	placement, err := parsePlacementConstraint(input.Region)
	if err != nil {
		sendJSONError(w, err.Error(), "available: EU, EEA, US, DE, NR", http.StatusBadRequest)
		return
	}

	server.setGeofenceForUser(w, r, placement)
}

func (server *Server) disableBotRestriction(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	if user.Status != console.PendingBotVerification {
		sendJSONError(w, fmt.Sprintf("user with email %q must have PendingBotVerification status to disable bot restriction", userEmail),
			"", http.StatusBadRequest)
		return
	}

	err = server.freezeAccounts.BotUnfreezeUser(ctx, user.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if errs.Is(err, console.ErrNoFreezeStatus) {
			status = http.StatusConflict
		}
		sendJSONError(w, "failed to unfreeze bot user", err.Error(), status)
	}
}

func (server *Server) deleteGeofenceForAccount(w http.ResponseWriter, r *http.Request) {
	server.setGeofenceForUser(w, r, storj.DefaultPlacement)
}

func (server *Server) setGeofenceForUser(w http.ResponseWriter, r *http.Request, placement storj.PlacementConstraint) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing", "", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user details",
			err.Error(), http.StatusInternalServerError)
		return
	}

	if user.DefaultPlacement == placement {
		sendJSONError(w, "new placement is equal to user's current placement",
			"", http.StatusBadRequest)
		return
	}

	if err = server.db.Console().Users().UpdateDefaultPlacement(ctx, user.ID, placement); err != nil {
		sendJSONError(w, "unable to set geofence for user",
			err.Error(), http.StatusInternalServerError)
		return
	}
}

func (server *Server) updateFreeTrialExpiration(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		sendJSONError(w, "user-email missing",
			"", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		sendJSONError(w, fmt.Sprintf("user with email %q does not exist", userEmail),
			"", http.StatusNotFound)
		return
	}
	if err != nil {
		sendJSONError(w, "failed to get user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendJSONError(w, "failed to read body",
			err.Error(), http.StatusInternalServerError)
		return
	}

	var input struct {
		TrialExpiration *time.Time `json:"trialExpiration"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		sendJSONError(w, "failed to unmarshal request",
			err.Error(), http.StatusBadRequest)
		return
	}

	expirationPtr := input.TrialExpiration
	err = server.db.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{TrialExpiration: &expirationPtr})
	if err != nil {
		sendJSONError(w, "failed to update user",
			err.Error(), http.StatusInternalServerError)
		return
	}
}
