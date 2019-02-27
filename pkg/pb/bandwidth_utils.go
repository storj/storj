// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	"bytes"
	reflect "reflect"

	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
)

//OrderLimit redefines PayerBandwidthAllocation (to allow fancy serialization)
type OrderLimit = PayerBandwidthAllocation

//Order redefines RenterBandwidthAllocation (to allow fancy serialization)
type Order = RenterBandwidthAllocation

//SignedHash redefines RenterBandwidthAllocation (to allow fancy serialization)
type SignedHash = []byte

var (
	// ErrRenter wraps errors related to renter bandwidth allocations
	ErrRenter = errs.Class("Renter agreement")
	// ErrPayer wraps errors related to payer bandwidth allocations
	ErrPayer = errs.Class("Payer agreement")
)

// Equal compares two Protobuf messages via serialization
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
	msg2Bytes, err := proto.Marshal(msg2)
	if err != nil {
		return false
	}
	return bytes.Compare(msg1Bytes, msg2Bytes) == 0
}

// Clone creates a deep copy of PayerBandwidthAllocation
func (m *OrderLimit) Clone() (pba OrderLimit) {
	pba = OrderLimit{
		SatelliteId:       m.SatelliteId,
		UplinkId:          m.UplinkId,
		MaxSize:           m.MaxSize,
		ExpirationUnixSec: m.ExpirationUnixSec,
		SerialNumber:      m.SerialNumber,
		Action:            m.Action,
		CreatedUnixSec:    m.CreatedUnixSec,
	}
	pba.Certs = make([][]byte, len(m.Certs))
	copy(pba.Certs, m.Certs)
	pba.Signature = make([]byte, len(m.Signature))
	copy(pba.Signature, m.Signature)

	return pba
}

//SignedMessageBase allows composition of signed messages
type SignedMessageBase func()

// Marshal serializes a node id
func (m SignedMessageBase) Marshal() ([]byte, error) {
	return id.Bytes(), nil
}

// MarshalTo serializes a node ID into the passed byte slice
func (m SignedMessageBase) MarshalTo(data []byte) (n int, err error) {
	n = copy(data, id.Bytes())
	return n, nil
}

// Unmarshal deserializes a node ID
func (m SignedMessageBase) Unmarshal(data []byte) error {
	var err error
	*id, err = NodeIDFromBytes(data)
	return err
}

// Size returns the length of a node ID (implements gogo's custom type interface)
func (m SignedMessageBase) Size() int {
	return len(id)
}

// MarshalJSON serializes a node ID to a json string as bytes
func (m SignedMessageBase) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON deserializes a json string (as bytes) to a node ID
func (m SignedMessageBase) UnmarshalJSON(data []byte) error {
	var err error
	*id, err = NodeIDFromString(string(data))
	if err != nil {
		return err
	}
	return nil
}
