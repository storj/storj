// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"errors"

	"storj.io/storj/pkg/identity"
)

//go:generate go run gen_identities.go -count 150 -out identities_table.go
//go:generate go run gen_identities.go -signed -count 150 -out signed_identities_table.go

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

// MustPregeneratedIdentity returns a pregenerated identity or panics
func MustPregeneratedIdentity(index int) *identity.FullIdentity {
	identity, err := PregeneratedIdentity(index)
	if err != nil {
		panic(err)
	}
	return identity
}

// MustPregeneratedSignedIdentity returns a pregenerated identity or panics
func MustPregeneratedSignedIdentity(index int) *identity.FullIdentity {
	identity, err := PregeneratedSignedIdentity(index)
	if err != nil {
		panic(err)
	}
	return identity
}

// PregeneratedIdentity returns a pregenerated identity from a list
func PregeneratedIdentity(index int) (*identity.FullIdentity, error) {
	if pregeneratedIdentities.next >= len(pregeneratedIdentities.list) {
		return nil, errors.New("out of pregenerated identities")
	}
	return pregeneratedIdentities.list[index], nil
}

// PregeneratedSignedIdentity returns a signed pregenerated identity from a list
func PregeneratedSignedIdentity(index int) (*identity.FullIdentity, error) {
	if pregeneratedIdentities.next >= len(pregeneratedSignedIdentities.list) {
		return nil, errors.New("out of signed pregenerated identities")
	}
	return pregeneratedSignedIdentities.list[index], nil
}

// NewPregeneratedIdentities retruns a new table from provided identities.
func NewPregeneratedIdentities() *Identities {
	return pregeneratedIdentities.Clone()
}

// NewPregeneratedSignedIdentities retruns a new table from provided signed identities.
func NewPregeneratedSignedIdentities() *Identities {
	return pregeneratedSignedIdentities.Clone()
}

// NewPregeneratedSigner returns the signer for all pregenerated, signed identities
func NewPregeneratedSigner() *identity.FullCertificateAuthority {
	return pregeneratedSigner
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

// mustParseIdentityPEM parses pem encoded identity chain and key strings.
func mustParseIdentityPEM(chain, key string) *identity.FullIdentity {
	// TODO: add whitelist handling somehow
	fi, err := identity.FullIdentityFromPEM([]byte(chain), []byte(key))
	if err != nil {
		panic(err)
	}
	return fi
}

// mustParseCertificateAuthorityPEM parses pem encoded certificate authority chain and key strings.
func mustParseCertificateAuthorityPEM(chain, key string) *identity.FullCertificateAuthority {
	// TODO: add whitelist handling somehow
	fi, err := identity.FullCertificateAuthorityFromPEM([]byte(chain), []byte(key))
	if err != nil {
		panic(err)
	}
	return fi
}
