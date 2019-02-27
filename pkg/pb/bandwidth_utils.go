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
	pba = OrderLimit{PBA: &PBA{
		SatelliteId:       m.SatelliteId,
		UplinkId:          m.UplinkId,
		MaxSize:           m.MaxSize,
		ExpirationUnixSec: m.ExpirationUnixSec,
		SerialNumber:      m.SerialNumber,
		Action:            m.Action,
		CreatedUnixSec:    m.CreatedUnixSec,
	}, SignedMessage: SignedMessage{
		Data:      m.Data,
		Signature: m.Signature,
		Certs:     m.Certs,
	}}
	pba.Certs = make([][]byte, len(m.Certs))
	copy(pba.Certs, m.Certs)
	pba.Signature = make([]byte, len(m.Signature))
	copy(pba.Signature, m.Signature)

	return pba
}
