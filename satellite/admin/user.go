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

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
	"storj.io/storj/satellite/payments"
)

func (server *Server) addUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
		return
	}

	var input console.CreateUser

	err = json.Unmarshal(body, &input)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request: %v", err), http.StatusBadRequest)
		return
	}

	switch {
	case input.Email == "":
		http.Error(w, "Email is not set", http.StatusBadRequest)
		return
	case input.Password == "":
		http.Error(w, "Password is not set", http.StatusBadRequest)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 0)
	if err != nil {
		http.Error(w, "Unable to save password hash", http.StatusInternalServerError)
		return
	}

	userID, err := uuid.New()
	if err != nil {
		http.Error(w, "Unable to create UUID", http.StatusInternalServerError)
		return
	}

	user := console.CreateUser{
		Email:    input.Email,
		FullName: input.FullName,
		Password: input.Password,
	}

	err = user.IsValid()
	if err != nil {
		http.Error(w, "User data is not valid", http.StatusBadRequest)
		return
	}

	newuser, err := server.db.Console().Users().Insert(ctx, &console.User{
		ID:           userID,
		FullName:     user.FullName,
		ShortName:    user.ShortName,
		Email:        user.Email,
		PasswordHash: hash,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to insert user: %v", err), http.StatusInternalServerError)
		return
	}
	//Set User Status to be activated, as we manually created it
	newuser.Status = console.Active
	newuser.PasswordHash = nil
	err = server.db.Console().Users().Update(ctx, newuser)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to activate user: %v", err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(newuser)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) userInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	userEmail, ok := vars["useremail"]
	if !ok {
		http.Error(w, "user-email missing", http.StatusBadRequest)
		return
	}

	user, err := server.db.Console().Users().GetByEmail(ctx, userEmail)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, fmt.Sprintf("user with email %q not found", userEmail), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user %q: %v", userEmail, err), http.StatusInternalServerError)
		return
	}
	user.PasswordHash = nil

	projects, err := server.db.Console().Projects().GetByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user projects %q: %v", userEmail, err), http.StatusInternalServerError)
		return
	}

	coupons, err := server.db.StripeCoinPayments().Coupons().ListByUserID(ctx, user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get user coupons %q: %v", userEmail, err), http.StatusInternalServerError)
		return
	}

	type User struct {
		ID       uuid.UUID `json:"id"`
		FullName string    `json:"fullName"`
		Email    string    `json:"email"`
	}
	type Project struct {
		ID          uuid.UUID `json:"id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		OwnerID     uuid.UUID `json:"ownerId"`
	}

	var output struct {
		User     User              `json:"user"`
		Projects []Project         `json:"projects"`
		Coupons  []payments.Coupon `json:"coupons"`
	}

	output.User = User{
		ID:       user.ID,
		FullName: user.FullName,
		Email:    user.Email,
	}
	for _, p := range projects {
		output.Projects = append(output.Projects, Project{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			OwnerID:     p.OwnerID,
		})
	}
	output.Coupons = coupons

	data, err := json.Marshal(output)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) addCoupon(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to read body: %v", err), http.StatusInternalServerError)
		return
	}

	var input struct {
		UserID      uuid.UUID `json:"userId"`
		Duration    int       `json:"duration"`
		Amount      int64     `json:"amount"`
		Description string    `json:"description"`
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to unmarshal request: %v", err), http.StatusBadRequest)
		return
	}
	switch {
	case input.Duration == 0:
		http.Error(w, "Duration is not set", http.StatusBadRequest)
		return
	case input.Amount == 0:
		http.Error(w, "Amount is not set", http.StatusBadRequest)
		return
	case input.UserID.IsZero():
		http.Error(w, "UserID is not set", http.StatusBadRequest)
		return
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Insert(ctx, payments.Coupon{
		UserID:      input.UserID,
		Amount:      input.Amount,
		Duration:    input.Duration,
		Description: input.Description,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to insert coupon: %v", err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}

func (server *Server) couponInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	id, ok := vars["couponid"]
	if !ok {
		http.Error(w, "couponId missing", http.StatusBadRequest)
		return
	}

	couponID, err := uuid.FromString(id)
	if err != nil {
		http.Error(w, "invalid couponId", http.StatusBadRequest)
	}

	coupon, err := server.db.StripeCoinPayments().Coupons().Get(ctx, couponID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, fmt.Sprintf("coupon with id %q not found", couponID), http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get coupon %q: %v", couponID, err), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(coupon)
	if err != nil {
		http.Error(w, fmt.Sprintf("json encoding failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data) // nothing to do with the error response, probably the client requesting disappeared
}
