// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"errors"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
)

//go:generate go run gen_identities.go -count 150 -out identities_table.go
//go:generate go run gen_identities.go -signed -count 150 -out signed_identities_table.go

const (
	MixedIndexesUnsigned = iota
	MixedIndexesSigned
)

var ErrMixedIdentityType = errs.New("unknown mixed identity indexes type")

// MixedIndexesTypeEnum is used to specify whether the identities at `mixedIndexes` are signed or not
type MixedIndexesTypeEnum int

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

// NewPregeneratedSignedIdentities retruns a new table from provided signed identities.
func NewPregeneratedSignedIdentities() *Identities {
	return pregeneratedSignedIdentities.Clone()
}

// NewPregeneratedSigner returns the signer for all pregenerated, signed identities
func NewPregeneratedSigner() *identity.FullCertificateAuthority {
	return pregeneratedSigner
}

// MixedIdentities returns a set of identities consisting of both signed and unsigned identities.
//	identities at `mixedIndexes` are either signed or not based on the value of `mixedIndexesType`
//	while all other identities are the respective opposite.
func MixedIdentities(mixedIndexes []int, mixedIndexesType MixedIndexesTypeEnum) (*Identities, error) {
	unsigned := NewPregeneratedIdentities()
	signed := NewPregeneratedSignedIdentities()

	var mixed *Identities
	replaceIdentities := func(original *Identities, indexes []int, replacement *Identities) error {
		for _, i := range indexes {
			var err error
			original.list[i], err = replacement.NewIdentity()
			if err != nil {
				return err
			}
		}
		return nil
	}

	switch mixedIndexesType {
	case MixedIndexesUnsigned:
		mixed = signed
		if err := replaceIdentities(mixed, mixedIndexes, unsigned); err != nil {
			return nil, err
		}
	case MixedIndexesSigned:
		mixed = unsigned
		if err := replaceIdentities(mixed, mixedIndexes, signed); err != nil {
			return nil, err
		}
	default:
		return nil, ErrMixedIdentityType
	}
	
	return mixed, nil
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
