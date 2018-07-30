package provider

import (
	"crypto/tls"
	"encoding/pem"
	"crypto/x509"
	"github.com/zeebo/errs"
	"encoding/base64"
	"go.uber.org/zap"
	"math/bits"
	"fmt"
	"storj.io/storj/pkg/peertls"
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

func certFromDERs(certDERBytes [][]byte, keyDERBytes []byte) (*tls.Certificate, error) {
	var (
		err  error
		cert = new(tls.Certificate)
	)

	cert.Certificate = certDERBytes
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}

	parsedLeaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cert.Leaf = parsedLeaf

	return cert, nil
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

			// TODO: connect peer verification function
			// kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

			if fi.Difficulty() >= difficulty {
				fiC <- fi
			}
		}
	}
}

// func VerifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
// 	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
// 		for _, certs := range parsedChains {
// 			for _, c := range certs {
// 				tc := &tls.Certificate{
// 					Certificate: rawChain,
// 				}
// 				pi, err := PeerIdentityFromCertChain(tc)
// 				if err != nil {
// 					return err
// 				}
//
// 				if pi.Difficulty() < difficulty {
// 					return ErrDifficulty.New("expected: %d; got: %d", difficulty, pi.Difficulty())
// 				}
// 			}
// 		}
//
// 		return nil
// 	}
// }
