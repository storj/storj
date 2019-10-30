// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"

	"github.com/zeebo/errs"
)

var (
	// NoMatchPartnerIDErr is the error class used when an offer has reached its redemption capacity
	NoMatchPartnerIDErr = errs.Class("partner not exist")
)

// GetPartnerID returns partner ID based on partner name
func GetPartnerID(partnerName string) (partnerID string, err error) {
	partner, err := DefaultPartnersDB.ByName(context.TODO(), partnerName) // TODO: replace with a service
	if err != nil {
		return "", err
	}
	return partner.ID, nil
}
