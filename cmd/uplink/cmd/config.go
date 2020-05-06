// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"io/ioutil"
	"strings"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"

	"storj.io/common/storj"
	libuplink "storj.io/storj/lib/uplink"
	"storj.io/uplink"
)

var mon = monkit.Package()

// ClientConfig is a configuration struct for the uplink that controls how
// to talk to the rest of the network.
type ClientConfig struct {
	DialTimeout time.Duration `help:"timeout for dials" default:"0h2m00s"`
}

// Config uplink configuration
type Config struct {
	AccessConfig
	Client ClientConfig
}

// AccessConfig holds information about which accesses exist and are selected.
type AccessConfig struct {
	Accesses map[string]string `internal:"true"`
	Access   string            `help:"the serialized access, or name of the access to use" default:"" basic-help:"true"`

	// used for backward compatibility
	Scopes map[string]string `internal:"true"` // deprecated
	Scope  string            `internal:"true"` // deprecated

	Legacy // Holds on to legacy configuration values
}

// Legacy holds deprecated configuration values
type Legacy struct {
	Client struct {
		APIKey        string `default:"" help:"the api key to use for the satellite (deprecated)" noprefix:"true" deprecated:"true"`
		SatelliteAddr string `releaseDefault:"127.0.0.1:7777" devDefault:"127.0.0.1:10000" help:"the address to use for the satellite (deprecated)" noprefix:"true"`
	}
	Enc struct {
		EncryptionKey     string `help:"the root key for encrypting the data which will be stored in KeyFilePath (deprecated)" setup:"true" deprecated:"true"`
		KeyFilepath       string `help:"the path to the file which contains the root key for encrypting the data (deprecated)" deprecated:"true"`
		EncAccessFilepath string `help:"the path to a file containing a serialized encryption access (deprecated)" deprecated:"true"`
	}
}

// normalize looks for usage of deprecated config values and sets the respective
// non-deprecated config values accordingly and returns them in a copy of the config.
func (a AccessConfig) normalize() (_ AccessConfig) {
	// fallback to scope if access not found
	if a.Access == "" {
		a.Access = a.Scope
	}

	if a.Accesses == nil {
		a.Accesses = make(map[string]string)
	}

	// fallback to scopes if accesses not found
	if len(a.Accesses) == 0 {
		for name, access := range a.Scopes {
			a.Accesses[name] = access
		}
	}

	return a
}

// GetAccess returns the appropriate access for the config.
func (a AccessConfig) GetAccess() (_ *libuplink.Scope, err error) {
	defer mon.Task()(nil)(&err)

	a = a.normalize()

	access, err := a.GetNamedAccess(a.Access)
	if err != nil {
		return nil, err
	}
	if access != nil {
		return access, nil
	}

	// Otherwise, try to load the access name as a serialized access.
	if access, err := libuplink.ParseScope(a.Access); err == nil {
		return access, nil
	}

	if len(a.Legacy.Client.APIKey) == 0 {
		return nil, errs.New("unable to find access grant, run 'setup' command or provide '--access' parameter")
	}

	// fall back to trying to load the legacy values.
	apiKey, err := libuplink.ParseAPIKey(a.Legacy.Client.APIKey)
	if err != nil {
		return nil, err
	}

	satelliteAddr := a.Legacy.Client.SatelliteAddr
	if satelliteAddr == "" {
		return nil, errs.New("must specify a satellite address")
	}

	var encAccess *libuplink.EncryptionAccess
	if a.Legacy.Enc.EncAccessFilepath != "" {
		data, err := ioutil.ReadFile(a.Legacy.Enc.EncAccessFilepath)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess, err = libuplink.ParseEncryptionAccess(strings.TrimSpace(string(data)))
		if err != nil {
			return nil, err
		}
	} else {
		data := []byte(a.Legacy.Enc.EncryptionKey)
		if a.Legacy.Enc.KeyFilepath != "" {
			data, err = ioutil.ReadFile(a.Legacy.Enc.KeyFilepath)
			if err != nil {
				return nil, errs.Wrap(err)
			}
		}
		key, err := storj.NewKey(data)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		encAccess = libuplink.NewEncryptionAccessWithDefaultKey(*key)
		encAccess.SetDefaultPathCipher(storj.EncAESGCM)
	}

	return &libuplink.Scope{
		APIKey:           apiKey,
		SatelliteAddr:    satelliteAddr,
		EncryptionAccess: encAccess,
	}, nil
}

// GetNewAccess returns the appropriate access for the config.
func (a AccessConfig) GetNewAccess() (_ *uplink.Access, err error) {
	defer mon.Task()(nil)(&err)

	oldAccess, err := a.GetAccess()
	if err != nil {
		return nil, err
	}

	serializedOldAccess, err := oldAccess.Serialize()
	if err != nil {
		return nil, err
	}

	access, err := uplink.ParseAccess(serializedOldAccess)
	if err != nil {
		return nil, err
	}
	return access, nil
}

// GetNamedAccess returns named access if exists.
func (a AccessConfig) GetNamedAccess(name string) (_ *libuplink.Scope, err error) {
	// if an access exists for that name, try to load it.
	if data, ok := a.Accesses[name]; ok {
		return libuplink.ParseScope(data)
	}
	return nil, nil
}

// IsSerializedAccess returns whether the passed access is a serialized
// access string or not.
func IsSerializedAccess(access string) bool {
	_, err := libuplink.ParseScope(access)
	return err == nil
}
