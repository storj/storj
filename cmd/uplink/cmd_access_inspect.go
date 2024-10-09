// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/zeebo/clingy"
	"github.com/zeebo/errs"

	"storj.io/common/base58"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/storj/cmd/uplink/ulext"
)

// ensures that cmdAccessInspect implements clingy.Command.
var _ clingy.Command = (*cmdAccessInspect)(nil)

// cmdAccessInspect is an access inspect command itself.
type cmdAccessInspect struct {
	ex     ulext.External
	access *string
}

// newCmdAccessInspect is a constructor for cmdAccessInspect.
func newCmdAccessInspect(ex ulext.External) clingy.Command {
	return &cmdAccessInspect{ex: ex}
}

// Setup is called to define and parse arguments.
func (c *cmdAccessInspect) Setup(params clingy.Parameters) {
	c.access = params.Arg("access", "Inspect access by its name or value.", clingy.Optional).(*string)
}

// Execute runs the command.
func (c *cmdAccessInspect) Execute(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	toOpen := ""
	if c.access != nil {
		toOpen = *c.access
	}

	access, err := c.ex.OpenAccess(toOpen)
	if err != nil {
		return err
	}

	serializedAccess, err := access.Serialize()
	if err != nil {
		return errs.New("could not serialize access: %+v", err)
	}

	p, err := parseAccessRaw(serializedAccess)
	if err != nil {
		return errs.New("could not parse access: %+v", err)
	}

	m, err := macaroon.ParseMacaroon(p.ApiKey)
	if err != nil {
		return errs.New("could not parse macaroon: %+v", err)
	}

	// TODO: this could be better
	apiKey, err := macaroon.ParseRawAPIKey(p.ApiKey)
	if err != nil {
		return errs.New("could not parse api key: %+v", err)
	}

	accessInspect := accessInspect{
		SatelliteAddr:    p.SatelliteAddr,
		EncryptionAccess: p.EncryptionAccess,
		APIKey:           apiKey.Serialize(),
		Macaroon: accessInspectMacaroon{
			Head:    m.Head(),
			Caveats: []macaroon.Caveat{},
			Tail:    m.Tail(),
		},
	}

	for _, cb := range m.Caveats() {
		var c macaroon.Caveat

		err = c.UnmarshalBinary(cb)
		if err != nil {
			return err
		}

		accessInspect.Macaroon.Caveats = append(accessInspect.Macaroon.Caveats, c)
	}

	bs, err := json.MarshalIndent(accessInspect, "", "  ")
	if err != nil {
		return err
	}

	_, _ = fmt.Fprintln(clingy.Stdout(ctx), string(bs))

	return nil
}

// parseAccessRaw decodes Scope from base58 string, that contains SatelliteAddress, ApiKey, and EncryptionAccess.
func parseAccessRaw(access string) (_ *pb.Scope, err error) {
	data, version, err := base58.CheckDecode(access)
	if err != nil || version != 0 {
		return nil, errs.New("invalid access grant format: %w", err)
	}

	p := new(pb.Scope)
	if err := pb.Unmarshal(data, p); err != nil {
		return nil, err
	}

	return p, nil
}

// accessInspect contains all info about access inspection that should be presented on cli.
type accessInspect struct {
	SatelliteAddr    string                `json:"satellite_addr"`
	EncryptionAccess *pb.EncryptionAccess  `json:"encryption_access"`
	APIKey           string                `json:"api_key"`
	Macaroon         accessInspectMacaroon `json:"macaroon"`
}

// base64url stores bytes representation of base64 encoded url.
type base64url []byte

// MarshalJSON implements the json.Marshaler interface for base64url.
func (b base64url) MarshalJSON() ([]byte, error) {
	return []byte(`"` + base64.URLEncoding.EncodeToString(b) + `"`), nil
}

// accessInspectMacaroon contains all info about access macaroon that should be presented on cli.
type accessInspectMacaroon struct {
	Head    base64url         `json:"head"`
	Caveats []macaroon.Caveat `json:"caveats"`
	Tail    base64url         `json:"tail"`
}
