// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import "github.com/zeebo/errs"

var (
	//Renter wraps errors related to renter bandwidth allocations
	Renter = errs.Class("Renter agreement")
	//Payer wraps errors related to payer bandwidth allocations
	Payer = errs.Class("Payer agreement")
)

//SetCerts updates the certs field, completing the auth.SignedMsg interface
func (m *PayerBandwidthAllocation) SetCerts(certs [][]byte) bool {
	if m != nil {
		m.Certs = certs
		return true
	}
	return false
}

//SetSignature updates the signature field, completing the auth.SignedMsg interface
func (m *PayerBandwidthAllocation) SetSignature(signature []byte) bool {
	if m != nil {
		m.Signature = signature
		return true
	}
	return false
}

//SetCerts updates the certs field, completing the auth.SignedMsg interface
func (m *RenterBandwidthAllocation) SetCerts(certs [][]byte) bool {
	if m != nil {
		m.Certs = certs
		return true
	}
	return false
}

//SetSignature updates the signature field, completing the auth.SignedMsg interface
func (m *RenterBandwidthAllocation) SetSignature(signature []byte) bool {
	if m != nil {
		m.Signature = signature
		return true
	}
	return false
}
