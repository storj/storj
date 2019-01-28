// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pb

import (
	proto "github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"
)

var (
	//Renter wraps errors related to renter bandwidth allocations
	Renter = errs.Class("Renter agreement")
	//Payer wraps errors related to payer bandwidth allocations
	Payer = errs.Class("Payer agreement")
	//Marshal indicates a failure during serialization
	Marshal = errs.Class("Could not generate byte array from key")
	//Unmarshal indicates a failure during deserialization
	Unmarshal = errs.Class("Could not generate key from byte array")
	//Missing indicates missing or empty information
	Missing = errs.Class("Required field is empty")
)

//SignedMsg interface has a key, data, and signature
type SignedMsg interface {
	GetCerts() [][]byte
	GetData() []byte
	GetSignature() []byte
}

// MsgComplete ensures a SignedMsg has no nulls
func MsgComplete(sm SignedMsg) (bool, error) {
	if sm == nil {
		return false, Missing.New("message")
	} else if sm.GetData() == nil {
		return false, Missing.New("message data")
	} else if sm.GetSignature() == nil {
		return false, Missing.New("message signature")
	} else if sm.GetCerts() == nil {
		return false, Missing.New("message certificates")
	}
	return true, nil
}

//Unpack helps get things out of a RenterBandwidthAllocation
func (rba *RenterBandwidthAllocation) Unpack() (*RenterBandwidthAllocation_Data, *PayerBandwidthAllocation, *PayerBandwidthAllocation_Data, error) {
	if ok, err := MsgComplete(rba); !ok {
		return nil, nil, nil, Renter.Wrap(err)
	}
	rbad := &RenterBandwidthAllocation_Data{}
	if err := proto.Unmarshal(rba.GetData(), rbad); err != nil {
		return nil, nil, nil, Renter.Wrap(Unmarshal.Wrap(err))
	}
	if ok, err := MsgComplete(rba); !ok {
		return nil, nil, nil, Payer.Wrap(err)
	}
	pba := rbad.GetPayerAllocation()
	pbad := &PayerBandwidthAllocation_Data{}
	if err := proto.Unmarshal(pba.GetData(), pbad); err != nil {
		return nil, nil, nil, Payer.Wrap(Unmarshal.Wrap(err))
	}
	return rbad, pba, pbad, nil
}
