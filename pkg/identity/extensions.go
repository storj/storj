// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package identity

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
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

// POWCounterFromCert retrieves the POWCounter from the certificate's
// proof-of-work counter extension value.
func POWCounterFromCert(cert *x509.Certificate) (peertls.POWCounter, error) {
	exts := extensions.NewExtensionsMap(cert)
	counterExt, ok := exts[extensions.IdentityPOWCounterExtID.String()]
	if !ok {
		return 0, Error.New("no proof-of-work counter extension found")
	}
	counterLen := len(counterExt.Value)
	if counterLen != 8 {
		return 0, Error.New("invalid counter extension length %d", counterLen)
	}
	return peertls.POWCounter(binary.BigEndian.Uint64(counterExt.Value)), nil
}
