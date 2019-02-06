// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"bytes"
	reflect "reflect"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
)

var (
	//ErrRenter wraps errors related to renter bandwidth allocations
	ErrRenter = errs.Class("Renter agreement")
	//ErrPayer wraps errors related to payer bandwidth allocations
	ErrPayer = errs.Class("Payer agreement")
)

//Equal compares two Protobuf messages via serialization
func Equal(msg1, msg2 proto.Message) bool {
	//reflect.DeepEqual and proto.Equal don't seem work in all cases
	//todo:  see how slow this is compared to custom equality checks
	if msg1 == nil {
		return msg2 == nil
	}
	if reflect.TypeOf(msg1) != reflect.TypeOf(msg2) {
		return false
	}
	msg1Bytes, err := proto.Marshal(msg1)
	if err != nil {
		return false
	}
	msg2Bytes, err := proto.Marshal(msg1)
	if err != nil {
		return false
	}
	return bytes.Compare(msg1Bytes, msg2Bytes) == 0
}

//SafeMarshal deterministically generates bytes
func (m *RenterBandwidthAllocation) SafeMarshal() ([]byte, error) {
	//todo: proto.Marshal isn't deterministic if we ever use Maps
	return proto.Marshal(m)
}

//SetSignature updates the signature field, completing the auth.SignedMsg interface
func (m *PayerBandwidthAllocation) SetSignature(signature []byte) {
	m.Signature = signature
}

//SafeMarshal deterministically generates bytes
func (m *PayerBandwidthAllocation) SafeMarshal() ([]byte, error) {
	//todo: proto.Marshal isn't deterministic if we ever use Maps
	return proto.Marshal(m)
}

//SetSignature updates the signature field, completing the auth.SignedMsg interface
func (m *RenterBandwidthAllocation) SetSignature(signature []byte) {
	m.Signature = signature
}
