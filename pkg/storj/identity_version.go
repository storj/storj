// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"strconv"
	"strings"

	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/pkcrypto"
)

const (
	V1 = IDVersionNumber(iota + 1)
	V2
)

var (
	IDVersions = map[IDVersionNumber]IDVersion{
		/* V1 breaking change:
		+ removed support for difficulties < 9
		*/
		V1: {
			Number:        V1,
			NewPrivateKey: pkcrypto.GeneratePrivateKey,
		},
		/* V2 changes:
		+ add version support
		+ change elliptic curve to non-NIST
		*/
		V2: {
			Number:        V2,
			NewPrivateKey: pkcrypto.GeneratePrivateKey,
			// TODO@thepaul: update this
		},
	}

	IDVersionHandler = extensions.NewHandlerFactory(
		&extensions.IdentityVersionExtID, idVersionHandler,
	)
)

type IDVersionNumber uint8

type IDVersion struct {
	Number        IDVersionNumber
	NewPrivateKey func() (crypto.PrivateKey, error)
}

func init() {
	extensions.AllHandlers.Register(IDVersionHandler)
}

func GetIDVersion(number IDVersionNumber) (IDVersion, error) {
	if number == 0 {
		return LatestIDVersion(), nil
	}

	version, ok := IDVersions[number]
	if !ok {
		return IDVersion{}, ErrVersion.New("unknown version")
	}

	return version, nil
}

func LatestIDVersion() IDVersion {
	return IDVersions[IDVersionNumber(len(IDVersions))]
}

func IDVersionFromCert(cert *x509.Certificate) (IDVersion, error) {
	for _, ext := range cert.ExtraExtensions {
		if extensions.IdentityVersionExtID.ToASN1().Equal(ext.Id) {
			return GetIDVersion(IDVersionNumber(ext.Value[0]))
		}
	}
	return IDVersion{}, ErrVersion.New("certificate doesn't contain an identity version extension")
}

func IDVersionInVersions(versionNumber IDVersionNumber, versionsStr string) error {
	switch versionsStr {
	case "latest":
		if versionNumber == LatestIDVersion().Number {
			return nil
		}
	default:
		versionRanges := strings.Split(versionsStr, ",")
		for _, versionRange := range versionRanges {
			if strings.Contains(versionRange, "-") {
				versionLimits := strings.Split(versionRange, "-")
				if len(versionLimits) != 2 {
					return ErrVersion.New("malformed PeerIDVersions string: %s", versionsStr)
				}

				begin, err := strconv.Atoi(versionLimits[0])
				if err != nil {
					return ErrVersion.Wrap(err)
				}

				end, err := strconv.Atoi(versionLimits[1])
				if err != nil {
					return ErrVersion.Wrap(err)
				}

				for i := begin; i <= end; i ++ {
					if versionNumber == IDVersionNumber(i) {
						return nil
					}
				}
			} else {
				versionInt, err := strconv.Atoi(versionRange)
				if err != nil {
					return ErrVersion.Wrap(err)
				}
				if versionNumber == IDVersionNumber(versionInt) {
					return nil
				}
			}
		}
	}
	return ErrVersion.New("version %d not in versions %s", versionNumber, versionsStr)
}

// TODO: should this include signature?
func AddVersionExt(version IDVersionNumber, cert *x509.Certificate) error {
	return extensions.AddExtension(cert, pkix.Extension{
		Id:    extensions.IdentityVersionExtID.ToASN1(),
		Value: []byte{byte(version)},
	})
}

func idVersionHandler(opts *extensions.Options) extensions.HandlerFunc {
	return func(ext pkix.Extension, chain [][]*x509.Certificate) error {
		return IDVersionInVersions(IDVersionNumber(ext.Value[0]), opts.PeerIDVersions)
	}
}
