// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package rewards

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/zeebo/errs"
)

// PartnersList defines a json struct for defining partners.
type PartnersList struct {
	Partners []Partner
}

// Partner contains information about a partner.
type Partner struct {
	Name string
	ID   string
}

// UserAgent returns partners cano user agent.
func (p *Partner) UserAgent() string { return p.Name }

// CanonicalUserAgentProduct returns canonicalizes the user agent product, which is suitable for lookups.
func CanonicalUserAgentProduct(product string) string { return strings.ToLower(product) }

// PartnersListFromJSONFile loads a json definition of partners.
func PartnersListFromJSONFile(path string) (*PartnersList, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(file.Close()))
	}()

	var list PartnersList
	err = json.NewDecoder(file).Decode(&list)
	return &list, Error.Wrap(err)
}
