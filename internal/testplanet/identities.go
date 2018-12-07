// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"errors"

	"storj.io/storj/pkg/provider"
)

//go:generate go run gen_identities.go -count 150 -out identities_table.go

// Identities is a pregenerated full identity table.
type Identities struct {
	list []*provider.FullIdentity
	next int
}

// NewIdentities creates a new table from provided identities.
func NewIdentities(list ...*provider.FullIdentity) *Identities {
	return &Identities{
		list: list,
		next: 0,
	}
}

// Clone creates a shallow clone of the table.
func (identities *Identities) Clone() *Identities {
	return NewIdentities(identities.list...)
}

// NewIdentity gets a new identity from the list.
func (identities *Identities) NewIdentity() (*provider.FullIdentity, error) {
	if identities.next >= len(identities.list) {
		return nil, errors.New("out of pregenerated identities")
	}

	id := identities.list[identities.next]
	identities.next++
	return id, nil
}

// mustParsePEM parses pem encoded chain and key strings.
func mustParsePEM(chain, key string) *provider.FullIdentity {
	// TODO: add whitelist handling somehow
	fi, err := provider.FullIdentityFromPEM([]byte(chain), []byte(key))
	if err != nil {
		panic(err)
	}
	return fi
}
