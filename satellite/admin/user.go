// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

func (server *Server) addUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
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

	err = user.IsValid()
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

	newuser, err := server.db.Console().Users().Insert(ctx, &console.User{
		ID:                    userID,
		FullName:              user.FullName,
		ShortName:             user.ShortName,
		Email:                 user.Email,
		PasswordHash:          hash,
		ProjectLimit:          server.config.ConsoleConfig.DefaultProjectLimit,
		ProjectStorageLimit:   server.config.ConsoleConfig.UsageLimits.Storage.Free.Int64(),
		ProjectBandwidthLimit: server.config.ConsoleConfig.UsageLimits.Bandwidth.Free.Int64(),
		SignupPromoCode:       user.SignupPromoCode,
	})
	if err != nil {
		sendJSONError(w, "failed to insert user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = server.payments.Setup(ctx, newuser.ID, newuser.Email, newuser.SignupPromoCode)
	if err != nil {
		sendJSONError(w, "failed to create payment account for user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	// Set User Status to be activated, as we manually created it
	newuser.Status = console.Active
	newuser.PasswordHash = nil
	err = server.db.Console().Users().Update(ctx, newuser)
	if err != nil {
		sendJSONError(w, "failed to activate user",
			err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(newuser)
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

	projects, err := server.db.Console().Projects().GetByUserID(ctx, user.ID)
	if err != nil {
		sendJSONError(w, "failed to get user projects",
			err.Error(), http.StatusInternalServerError)
		return
	}

	type User struct {
		ID           uuid.UUID `json:"id"`
		FullName     string    `json:"fullName"`
		Email        string    `json:"email"`
		ProjectLimit int       `json:"projectLimit"`
	}
	type Project struct {
		ID          uuid.UUID `json:"id"`
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
	}
	for _, p := range projects {
		output.Projects = append(output.Projects, Project{
			ID:          p.ID,
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

	body, err := ioutil.ReadAll(r.Body)
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

	if input.FullName != "" {
		user.FullName = input.FullName
	}
	if input.ShortName != "" {
		user.ShortName = input.ShortName
	}
	if input.Email != "" {
		user.Email = input.Email
	}
	if !input.PartnerID.IsZero() {
		user.PartnerID = input.PartnerID
	}
	if len(input.PasswordHash) > 0 {
		user.PasswordHash = input.PasswordHash
	}
	if input.ProjectLimit > 0 {
		user.ProjectLimit = input.ProjectLimit
	}
	if input.ProjectStorageLimit > 0 {
		user.ProjectStorageLimit = input.ProjectStorageLimit
	}
	if input.ProjectBandwidthLimit > 0 {
		user.ProjectBandwidthLimit = input.ProjectBandwidthLimit
	}
	if input.ProjectSegmentLimit > 0 {
		user.ProjectSegmentLimit = input.ProjectSegmentLimit
	}
	if input.PaidTierStr != "" {
		status, err := strconv.ParseBool(input.PaidTierStr)
		if err != nil {
			sendJSONError(w, "failed to parse paid tier status",
				err.Error(), http.StatusBadRequest)
			return
		}

		user.PaidTier = status
	}

	err = server.db.Console().Users().Update(ctx, user)
	if err != nil {
		sendJSONError(w, "failed to update user",
			err.Error(), http.StatusInternalServerError)
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
	projects, err := server.db.Console().Projects().GetByUserID(ctx, user.ID)
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

	userInfo := &console.User{
		ID:        user.ID,
		FullName:  "",
		ShortName: "",
		Email:     fmt.Sprintf("deactivated+%s@storj.io", user.ID.String()),
		Status:    console.Deleted,
	}

	err = server.db.Console().Users().Update(ctx, userInfo)
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
