package provider

import (
	"crypto"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/bits"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/peertls"
)

type secretIdentity struct {
	FullIdentity
	crypto.PrivateKey
}

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
	if err != nil {
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
func generateCreds(difficulty uint16, siC chan secretIdentity, done chan bool) {
	for {
		select {
		case <-done:
			return
		default:
			leafCert, caCert, err := peertls.Generate()
			if err != nil {
				zap.S().Error(errs.Wrap(err))
				continue
			}

			pi, err := PeerIdentityFromCerts(leafCert.Leaf, caCert.Leaf)
			if err != nil {
				zap.S().Error(errs.Wrap(err))
				continue
			}

			si := secretIdentity{
				FullIdentity{
					*pi,
					leafCert.PrivateKey,
				},
				caCert.PrivateKey,
			}

			if si.FullIdentity.Difficulty() >= difficulty {
				siC <- si
			}
		}
	}
}

func VerifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
		// NB: use the first chain; leaf should be first, followed by the ca
		pi, err := PeerIdentityFromCerts(parsedChains[0][0], parsedChains[0][1])
		if err != nil {
			return err
		}

		if pi.Difficulty() < difficulty {
			return node.ErrDifficulty.New("expected: %d; got: %d", difficulty, pi.Difficulty())
		}

		return nil
	}
}
