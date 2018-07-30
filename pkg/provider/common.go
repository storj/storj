// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package provider

import (
	"github.com/zeebo/errs"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"math/bits"
	"fmt"
	"go.uber.org/zap"
	"encoding/base64"
	"storj.io/storj/pkg/peertls"
)

var (
	mon = monkit.Package()

	// Error is a provider error
	Error = errs.Class("provider error")
)

func idDifficulty(id nodeID) uint16 {
	hash, err := base64.URLEncoding.DecodeString(id.String())
	if err!= nil {
		zap.S().Error(errs.Wrap(err))
	}

	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			zeroBits := bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				zeroBits = 0
			}

			return uint16((i-1)*8 + zeroBits)
		}
	}

	// NB: this should never happen
	reason := fmt.Sprintf("difficulty matches hash length! hash: %s", hash)
	zap.S().Error(reason)
	panic(reason)
}

func generateCreds(difficulty uint16, fiC chan FullIdentity, done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			tlsH, _ := peertls.NewTLSHelper(nil)

			cert := tlsH.Certificate()
			pi, err := PeerIdentityFromCertChain(&cert)
			if err != nil {
				zap.S().Error(errs.Wrap(err))
				continue
			}

			fi := FullIdentity{
				PeerIdentity: *pi,
				PrivateKey: cert.PrivateKey,
			}

			// TODO: connect peer verification function
			// kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

			if fi.Difficulty() >= difficulty {
				fiC <- fi
			}
		}
	}
}
