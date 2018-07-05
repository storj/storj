package peertls

import (
	"crypto/ecdsa"
	"encoding/pem"
	"io"
	"os"

	"github.com/zeebo/errs"
)

func writePem(block *pem.Block, file io.WriteCloser) (_ error) {
	if err := pem.Encode(file, block); err != nil {
		return errs.New("unable to PEM-encode/write bytes to file", err)
	}

	return nil
}

func writeCerts(certs [][]byte, path string) (_ error) {
	file, err := os.Create(path)
	defer file.Close()

	if err != nil {
		return errs.New("unable to open file \"%s\" for writing", path, err)
	}

	for _, cert := range certs {
		block := newCertBlock(cert)
		if err := writePem(block, file); err != nil {
			return err
		}
	}

	if err := file.Close(); err != nil {
		return errs.New("unable to close file", err)
	}

	return nil
}

func writeKey(key *ecdsa.PrivateKey, path string) (_ error) {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open \"%s\" for writing", path, err)
	}

	block, err := keyToBlock(key)
	if err != nil {
		return err
	}

	if err := writePem(block, file); err != nil {
		return err
	}

	return nil
}
