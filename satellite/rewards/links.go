// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"path"

	"github.com/zeebo/errs"
)

var (
	// NoMatchPartnerIDErr is the error class used when an offer has reached its redemption capacity
	NoMatchPartnerIDErr = errs.Class("partner not exist")
)

// GeneratePartnerLink returns base64 encoded partner referral link
func GeneratePartnerLink(offerName string) ([]string, error) {
	pID, err := GetPartnerID(offerName)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	referralInfo := &referralInfo{UserID: "", PartnerID: pID}
	refJSON, err := json.Marshal(referralInfo)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	domains := getTardigradeDomains()
	referralLinks := make([]string, len(domains))
	encoded := base64.StdEncoding.EncodeToString(refJSON)

	for i, url := range domains {
		referralLinks[i] = path.Join(url, "ref", encoded)
	}

	return referralLinks, nil
}

// GetPartnerID returns partner ID based on partner name
func GetPartnerID(partnerName string) (partnerID string, err error) {
	partner, err := DefaultPartnersDB.ByName(context.TODO(), partnerName) // TODO: replace with a service
	if err != nil {
		return "", err
	}
	return partner.ID, nil
}
