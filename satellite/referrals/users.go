// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package referrals

import (
	"net/mail"

	"github.com/zeebo/errs"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/console"
)

// ErrValidation validation related error class.
var ErrValidation = errs.Class("validation error")

// CreateUser contains information that's necessary for creating a new user through referral program.
type CreateUser struct {
	FullName      string `json:"fullName"`
	ShortName     string `json:"shortName"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	ReferralToken string `json:"referralToken"`
}

// IsValid checks CreateUser validity and returns error describing whats wrong.
func (user *CreateUser) IsValid() error {
	var group errs.Group
	group.Add(console.ValidateFullName(user.FullName))
	group.Add(console.ValidatePassword(user.Password))

	// validate email
	_, err := mail.ParseAddress(user.Email)
	group.Add(err)

	if user.ReferralToken != "" {
		_, err := uuid.FromString(user.ReferralToken)
		group.Add(err)
	}

	return group.Err()
}
