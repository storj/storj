// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/skyrings/skyring-common/tools/uuid"
	"github.com/zeebo/errs"
)

// PartnerList defines a json struct for defining partners.
type PartnerList struct {
	Partners []PartnerInfo
}

// PartnerInfo contains information about a partner.
type PartnerInfo struct {
	Name string
	ID   string
	UUID *uuid.UUID
}

// UserAgent returns canonical user agent.
func (p *PartnerInfo) UserAgent() string { return p.Name }

// CanonicalUserAgentProduct returns canonicalizes the user agent product, which is suitable for lookups.
func CanonicalUserAgentProduct(product string) string { return strings.ToLower(product) }

// PartnersListFromJSONFile loads a json definition of partners.
func PartnersListFromJSONFile(path string) (*PartnerList, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, ErrPartners.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, ErrPartners.Wrap(file.Close()))
	}()

	var list PartnerList
	err = json.NewDecoder(file).Decode(&list)
	return &list, ErrPartners.Wrap(err)
}
