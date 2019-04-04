// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto/x509/pkix"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/storj"
)

// NewPOWCounterExt creates a new proof-of-work counter extension with the
// given counter value.
func NewPOWCounterExt(counter peertls.POWCounter) pkix.Extension {
	return pkix.Extension{
		Id: extensions.IdentityPOWCounterExtID,
		Value: counter.Bytes(),
	}
}

// NewVersionExt creates a new identity version certificate extension for the
// given identity version,
func NewVersionExt(version storj.IDVersion) pkix.Extension {
	return pkix.Extension{
		Id:    extensions.IdentityVersionExtID,
		Value: []byte{byte(version.Number)},
	}
}

