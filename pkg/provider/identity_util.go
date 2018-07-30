package provider

import (
	"encoding/pem"
	"crypto/x509"
	"github.com/zeebo/errs"
	"encoding/base64"
	"go.uber.org/zap"
	"math/bits"
	"fmt"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/node"
)

var (
	ErrZeroBytes = errs.New("byte slice was unexpectedly empty")
)

func decodePEM(PEMBytes []byte) ([][]byte, error) {
	DERBytes := [][]byte{}

	for {
		var DERBlock *pem.Block

		DERBlock, PEMBytes = pem.Decode(PEMBytes)
		if DERBlock == nil {
			break
		}

		DERBytes = append(DERBytes, DERBlock.Bytes)
	}

	if len(DERBytes) == 0 || len(DERBytes[0]) == 0 {
		return nil, ErrZeroBytes
}

	return DERBytes, nil
}

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

// TODO: resume here; change `fiC` channel to a struct that consists of
// `FullIdentity` and root private key
func generateCreds(difficulty uint16, fiC chan FullIdentity, done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			tlsH, _ := peertls.NewTLSHelper(nil)

			cert := tlsH.Certificate()
			pi, err := PeerIdentityFromCertChain(cert.Certificate)
			if err != nil {
				zap.S().Error(errs.Wrap(err))
				continue
			}

			fi := FullIdentity{
				PeerIdentity: *pi,
				PrivateKey: cert.PrivateKey,
			}

			if fi.Difficulty() >= difficulty {
				fiC <- fi
			}
		}
	}
}

func VerifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
		pi, err := PeerIdentityFromCertChain(rawChain)
		if err != nil {
			return err
		}

		if pi.Difficulty() < difficulty {
			return node.ErrDifficulty.New("expected: %d; got: %d", difficulty, pi.Difficulty())
		}

		return nil
	}
}
