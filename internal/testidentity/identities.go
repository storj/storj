// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testidentity

import (
	"errors"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

//go:generate go run gen_identities.go -version 1 -count 150 -out V1_identities_table.go
//go:generate go run gen_identities.go -version 2 -count 150 -out V2_identities_table.go
//go:generate go run gen_identities.go -signed -version 1 -count 150 -out V1_signed_identities_table.go
//go:generate go run gen_identities.go -signed -version 2 -count 150 -out V2_signed_identities_table.go

var (
	// IdentityVersions holds pregenerated identities for each/ identity version.
	IdentityVersions = VersionedIdentitiesMap{
		storj.V1: pregeneratedV1Identities,
		storj.V2: pregeneratedV2Identities,
	}

	// SignedIdentityVersions holds pregenerated, signed identities for each.
	// identity version
	SignedIdentityVersions = VersionedIdentitiesMap{
		storj.V1: pregeneratedV1SignedIdentities,
		storj.V2: pregeneratedV2SignedIdentities,
	}

	// SignerVersions holds certificate authorities for each identity version.
	SignerVersions = VersionedCertificateAuthorityMap{
		storj.V1: pregeneratedV1Signer,
		storj.V2: pregeneratedV2Signer,
	}
)

// VersionedIdentitiesMap maps a `storj.IDVersionNumber` to a set of
// pregenerated identities with the corresponding version.
type VersionedIdentitiesMap map[storj.IDVersionNumber]*Identities

// VersionedCertificateAuthorityMap maps a `storj.IDVersionNumber` to a set of
// pregenerated certificate authorities used for signing the corresponding
// version of signed identities.
type VersionedCertificateAuthorityMap map[storj.IDVersionNumber]*identity.FullCertificateAuthority

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
func MustPregeneratedIdentity(index int, version storj.IDVersion) *identity.FullIdentity {
	identity, err := PregeneratedIdentity(index, version)
	if err != nil {
		panic(err)
	}
	return identity
}

// MustPregeneratedSignedIdentity returns a pregenerated identity or panics
func MustPregeneratedSignedIdentity(index int, version storj.IDVersion) *identity.FullIdentity {
	identity, err := PregeneratedSignedIdentity(index, version)
	if err != nil {
		panic(err)
	}
	return identity
}

// PregeneratedIdentity returns a pregenerated identity from a list
func PregeneratedIdentity(index int, version storj.IDVersion) (*identity.FullIdentity, error) {
	pregeneratedIdentities := IdentityVersions[version.Number]

	if pregeneratedIdentities.next >= len(pregeneratedIdentities.list) {
		return nil, errors.New("out of pregenerated identities")
	}
	return pregeneratedIdentities.list[index], nil
}

// PregeneratedSignedIdentity returns a signed pregenerated identity from a list
func PregeneratedSignedIdentity(index int, version storj.IDVersion) (*identity.FullIdentity, error) {
	pregeneratedSignedIdentities := SignedIdentityVersions[version.Number]

	if pregeneratedSignedIdentities.next >= len(pregeneratedSignedIdentities.list) {
		return nil, errors.New("out of signed pregenerated identities")
	}
	return pregeneratedSignedIdentities.list[index], nil
}

// NewPregeneratedIdentities retruns a new table from provided identities.
func NewPregeneratedIdentities(version storj.IDVersion) *Identities {
	return IdentityVersions[version.Number].Clone()
}

// NewPregeneratedSignedIdentities retruns a new table from provided signed identities.
func NewPregeneratedSignedIdentities(version storj.IDVersion) *Identities {
	return SignedIdentityVersions[version.Number].Clone()
}

// NewPregeneratedSigner returns the signer for all pregenerated, signed identities
func NewPregeneratedSigner(version storj.IDVersion) *identity.FullCertificateAuthority {
	//return pregeneratedV1Signer
	return SignerVersions[version.Number]
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
