// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals

import (
	"net/mail"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"

	"storj.io/storj/satellite/console"
)

// ErrValidation validation related error class
var ErrValidation = errs.Class("validation error")

// CreateUser contains information that's necessary for creating a new user through referral program
type CreateUser struct {
	FullName      string `json:"fullName"`
	ShortName     string `json:"shortName"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	ReferralToken string `json:"referralToken"`
}

// IsValid checks CreateUser validity and returns error describing whats wrong.
func (user *CreateUser) IsValid() error {
	var errors []error

	errors = append(errors, console.ValidateFullName(user.FullName))
	errors = append(errors, console.ValidatePassword(user.Password))

	// validate email
	_, err := mail.ParseAddress(user.Email)
	errors = append(errors, err)

	if user.ReferralToken != "" {
		_, err := uuid.Parse(user.ReferralToken)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return errs.Combine(errors...)
}
