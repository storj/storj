// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"errors"

	"storj.io/storj/pkg/identity"
)

//go:generate go run gen_identities.go -count 150 -out identities_table.go

// Identities is a pregenerated full identity table.
type Identities struct {
	list []*identity.FullIdentity
	next int
}

// NewIdentities creates a new table from provided identities.
func NewIdentities(list ...*identity.FullIdentity) *Identities {
	return &Identities{
		list: list,
		next: 0,
	}
}

// PregeneratedIdentity returns a pregenerated identity from a list
func PregeneratedIdentity(index int) (*identity.FullIdentity, error) {
	if pregeneratedIdentities.next >= len(pregeneratedIdentities.list) {
		return nil, errors.New("out of pregenerated identities")
	}
	return pregeneratedIdentities.list[index], nil
}

// NewPregeneratedIdentities retruns a new table from provided identities.
func NewPregeneratedIdentities() *Identities {
	return pregeneratedIdentities.Clone()
}

// Clone creates a shallow clone of the table.
func (identities *Identities) Clone() *Identities {
	return NewIdentities(identities.list...)
}

// NewIdentity gets a new identity from the list.
func (identities *Identities) NewIdentity() (*identity.FullIdentity, error) {
	if identities.next >= len(identities.list) {
		return nil, errors.New("out of pregenerated identities")
	}

	id := identities.list[identities.next]
	identities.next++
	return id, nil
}

// mustParsePEM parses pem encoded chain and key strings.
func mustParsePEM(chain, key string) *identity.FullIdentity {
	// TODO: add whitelist handling somehow
	fi, err := identity.FullIdentityFromPEM([]byte(chain), []byte(key))
	if err != nil {
		panic(err)
	}
	return fi
}
