// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

// func verifyPeerIdentityFunc(difficulty uint16) peertls.PeerCertVerificationFunc {
// 	return func(rawChain [][]byte, parsedChains [][]*x509.Certificate) error {
// 		for _, certs := range parsedChains {
// 			for _, c := range certs {
// 				tc := &tls.Certificate{
// 					Certificate: rawChain,
// 				}
// 				pi, err := provider.PeerIdentityFromCertChain(tc)
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
//
// func baseConfig(difficulty, hashLen uint16) *tls.Config {
// 	return &tls.Config{
// 		VerifyPeerCertificate: verifyPeerIdentityFunc(difficulty),
// 	}
// }
//
// func generateCreds(difficulty, hashLen uint16, c chan provider.FullIdentity, done chan bool) {
// 	for {
// 		select {
// 		case <-done:
//
// 			return
// 		default:
// 			tlsH, _ := peertls.NewTLSHelper(nil)
//
// 			cert := tlsH.Certificate()
// 			kadCreds, _ := CertToCreds(&cert, hashLen)
// 			kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)
//
// 			if kadCreds.Difficulty() >= difficulty {
// 				c <- *kadCreds
// 			}
// 		}
// 	}
// }
//
// func pubKeyToPeerIdentity(pubKey []byte) (*PeerIdentity, error) {
// 	hashBytes, err := hash(pubKey, defaultHashLength)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return &PeerIdentity{
// 		pubKey:  pubKey,
// 		hash:    hashBytes,
// 	}, nil
// }
//
// func idBytes(pubKey []byte) []byte {
// 	b := bytes.NewBuffer([]byte{})
// 	encoder := base64.NewEncoder(base64.URLEncoding, b)
// 	if _, err := encoder.Write(pubKey); err != nil {
// 		zap.S().Error(errs.Wrap(err))
// 	}
//
// 	if err := encoder.Close(); err != nil {
// 		zap.S().Error(errs.Wrap(err))
// 	}
//
// 	return b.Bytes()
// }
//
// func idDifficulty(hash []byte) uint16 {
// 	for i := 1; i < len(hash); i++ {
// 		b := hash[len(hash)-i]
//
// 		if b != 0 {
// 			zeroBits := bits.TrailingZeros16(uint16(b))
// 			if zeroBits == 16 {
// 				zeroBits = 0
// 			}
//
// 			return uint16((i-1)*8 + zeroBits)
// 		}
// 	}
//
// 	// NB: this should never happen
// 	reason := fmt.Sprintf("difficulty matches hash length! hash: %s", hash)
// 	zap.S().Error(reason)
// 	panic(reason)
// }
//
// func (c *FullIdentity) writeRootKey(dir string) error {
// 	path := filepath.Join(filepath.Dir(dir), "root.pem")
// 	rootKey := c.tlsH.RootKey()
//
// 	if rootKey != (ecdsa.PrivateKey{}) {
// 		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
// 		if err != nil {
// 			return errs.New("unable to open identity file for writing \"%s\"", path, err)
// 		}
//
// 		defer func() {
// 			if err := file.Close(); err != nil {
// 				zap.S().Error(errs.Wrap(err))
// 			}
// 		}()
//
// 		keyBytes, err := peertls.KeyToDERBytes(&rootKey)
// 		if err != nil {
// 			return err
// 		}
//
// 		if err := pem.Encode(file, peertls.NewKeyBlock(keyBytes)); err != nil {
// 			return errs.Wrap(err)
// 		}
//
// 		c.tlsH.DeleteRootKey()
// 	}
//
// 	return nil
// }
//
// func (c *FullIdentity) write(writer io.Writer) error {
// 	for _, c := range c.tlsH.Certificate().Certificate {
// 		certBlock := peertls.NewCertBlock(c)
//
// 		if err := pem.Encode(writer, certBlock); err != nil {
// 			return errs.Wrap(err)
// 		}
// 	}
//
// 	keyDERBytes, err := peertls.KeyToDERBytes(
// 		c.tlsH.Certificate().PrivateKey.(*ecdsa.PrivateKey),
// 	)
// 	if err != nil {
// 		return err
// 	}
//
// 	if err := pem.Encode(writer, peertls.NewKeyBlock(keyDERBytes)); err != nil {
// 		return errs.Wrap(err)
// 	}
//
// 	return nil
// }
//
// func read(PEMBytes []byte) (*tls.Certificate, error) {
// 	certDERs := [][]byte{}
// 	keyDER := []byte{}
//
// 	for {
// 		var DERBlock *pem.Block
//
// 		DERBlock, PEMBytes = pem.Decode(PEMBytes)
// 		if DERBlock == nil {
// 			break
// 		}
//
// 		switch DERBlock.Type {
// 		case peertls.BlockTypeCertificate:
// 			certDERs = append(certDERs, DERBlock.Bytes)
// 			continue
//
// 		case peertls.BlockTypeEcPrivateKey:
// 			keyDER = DERBlock.Bytes
// 			continue
// 		}
// 	}
//
// 	if len(certDERs) == 0 || len(certDERs[0]) == 0 {
// 		return nil, errs.New("no certificates found in identity file")
// 	}
//
// 	if len(keyDER) == 0 {
// 		return nil, errs.New("no private key found in identity file")
// 	}
//
// 	cert, err := certFromDERs(certDERs, keyDER)
// 	if err != nil {
// 		return nil, errs.Wrap(err)
// 	}
//
// 	return cert, nil
// }
//
// // func certFromDERs(certDERBytes [][]byte, keyDERBytes []byte) (*tls.Certificate, error) {
// // 	var (
// // 		err  error
// // 		cert = new(tls.Certificate)
// // 	)
// //
// // 	cert.Certificate = certDERBytes
// // 	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
// // 	if err != nil {
// // 		return nil, errs.New("unable to parse EC private key", err)
// // 	}
// //
// // 	parsedLeaf, err := x509.ParseCertificate(cert.Certificate[0])
// // 	if err != nil {
// // 		return nil, errs.Wrap(err)
// // 	}
// //
// // 	cert.Leaf = parsedLeaf
// //
// // 	return cert, nil
// // }
//
// func hash(input []byte, hashLen uint16) ([]byte, error) {
// 	shake := sha3.NewShake256()
// 	if _, err := shake.Write(input); err != nil {
// 		return nil, errs.Wrap(err)
// 	}
//
// 	hashBytes := make([]byte, hashLen)
//
// 	bytesRead, err := shake.Read(hashBytes)
// 	if err != nil {
// 		return nil, errs.Wrap(err)
// 	}
//
// 	if uint16(bytesRead) != hashLen {
// 		return nil, errs.New("hash length error")
// 	}
//
// 	return hashBytes, nil
// }
