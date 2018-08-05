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

	"context"

	"golang.org/x/crypto/sha3"
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

func generateCAWorker(ctx context.Context, difficulty uint16, caC chan CertificateAuthority) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			ct, err := peertls.CATemplate()
			if err != nil {
				zap.S().Error(err)
				continue
			}

			c, err := peertls.Generate(ct, nil, nil, nil)
			if err != nil {
				zap.S().Error(err)
				continue
			}

			i, err := idFromCert(c.Leaf)
			if err != nil {
				zap.S().Error(err)
				continue
			}

			ca := CertificateAuthority{
				Cert: c.Leaf,
				Key:  &c.PrivateKey,
				ID:   i,
			}

			if ca.Difficulty() >= difficulty {
				caC <- ca
			}
		}
	}
}

func idFromCert(c *x509.Certificate) (nodeID, error) {
	caPublicKey, err := x509.MarshalPKIXPublicKey(c.PublicKey)
	if err != nil {
		return "", errs.Wrap(err)
	}

	hash := make([]byte, IdentityLength)
	sha3.ShakeSum256(hash, caPublicKey)

	return nodeID(base64.URLEncoding.EncodeToString(hash)), nil
}

func VerifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
		// NB: use the first chain; leaf should be first, followed by the ca
		pi, err := PeerIdentityFromCerts(parsedChains[0][0], parsedChains[0][1])
		if err != nil {
			return err
		}

		if pi.Difficulty() < difficulty {
			return ErrDifficulty.New("expected: \"%d\" but got: \"%d\"", difficulty, pi.Difficulty())
		}

		return nil
	}
}
