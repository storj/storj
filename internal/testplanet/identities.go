// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet

import (
	"errors"

	"storj.io/storj/pkg/provider"
)

//go:generate go run gen_identities.go -count 150 -out identities_table.go

type Identities struct {
	list []*provider.FullIdentity
	next int
}

func NewIdentities(list ...*provider.FullIdentity) *Identities {
	return &Identities{
		list: list,
		next: 0,
	}
}

func (identities *Identities) Clone() *Identities {
	return NewIdentities(identities.list...)
}

func (identities *Identities) NewIdentity() (*provider.FullIdentity, error) {
	if identities.next >= len(identities.list) {
		return nil, errors.New("out of pregenerated identities")
	}

	id := identities.list[identities.next]
	identities.next++
	return id, nil
}

func mustParsePEM(chain, key string) *provider.FullIdentity {
	// TODO: whitelist
	fi, err := provider.FullIdentityFromPEM([]byte(chain), []byte(key), nil)
	if err != nil {
		panic(err)
	}
	return fi
}
