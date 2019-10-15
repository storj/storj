// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package partners

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/zeebo/errs"
)

// List defines a json struct for defining partners.
type List struct {
	Partners []Partner
}

// Partner contains information about a partner.
type Partner struct {
	Name string
	ID   string
}

// UserAgent returns partners cano user agent.
func (p *Partner) UserAgent() string { return p.Name }

// CanonicalUserAgent returns canonicalizes the user name, which is suitable for lookups.
func CanonicalUserAgent(useragent string) string { return strings.ToLower(useragent) }

// ListFromJSONFile loads a json definition of partners.
func ListFromJSONFile(path string) (*List, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, Error.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, Error.Wrap(file.Close()))
	}()

	var list List
	err = json.NewDecoder(file).Decode(&list)
	return &list, Error.Wrap(err)
}
