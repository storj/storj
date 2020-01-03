// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"github.com/btcsuite/btcutil/base58"
	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/common/pb"
)

// Scope is a serializable type that represents all of the credentials you need
// to open a project and some amount of buckets
type Scope struct {
	SatelliteAddr string

	APIKey APIKey

	EncryptionAccess *EncryptionAccess
}

// ParseScope unmarshals a base58 encoded scope protobuf and decodes
// the fields into the Scope convenience type. It will return an error if the
// protobuf is malformed or field validation fails.
func ParseScope(scopeb58 string) (*Scope, error) {
	data, version, err := base58.CheckDecode(scopeb58)
	if err != nil || version != 0 {
		return nil, errs.New("invalid scope format")
	}

	p := new(pb.Scope)
	if err := proto.Unmarshal(data, p); err != nil {
		return nil, errs.New("unable to unmarshal scope: %v", err)
	}

	if len(p.SatelliteAddr) == 0 {
		return nil, errs.New("scope missing satellite URL")
	}

	apiKey, err := parseRawAPIKey(p.ApiKey)
	if err != nil {
		return nil, errs.New("scope has malformed api key: %v", err)
	}

	access, err := parseEncryptionAccessFromProto(p.EncryptionAccess)
	if err != nil {
		return nil, errs.New("scope has malformed encryption access: %v", err)
	}

	return &Scope{
		SatelliteAddr:    p.SatelliteAddr,
		APIKey:           apiKey,
		EncryptionAccess: access,
	}, nil
}

// Serialize serializes a Scope to a base58-encoded string
func (s *Scope) Serialize() (string, error) {
	switch {
	case len(s.SatelliteAddr) == 0:
		return "", errs.New("scope missing satellite URL")
	case s.APIKey.IsZero():
		return "", errs.New("scope missing api key")
	case s.EncryptionAccess == nil:
		return "", errs.New("scope missing encryption access")
	}

	access, err := s.EncryptionAccess.toProto()
	if err != nil {
		return "", err
	}

	data, err := proto.Marshal(&pb.Scope{
		SatelliteAddr:    s.SatelliteAddr,
		ApiKey:           s.APIKey.serializeRaw(),
		EncryptionAccess: access,
	})
	if err != nil {
		return "", errs.New("unable to marshal scope: %v", err)
	}

	return base58.CheckEncode(data, 0), nil
}
