// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/ecdsa"
	"encoding/pem"
	"io"
	"log"
	"os"

	"github.com/zeebo/errs"
)

func writePem(block *pem.Block, file io.WriteCloser) error {
	if err := pem.Encode(file, block); err != nil {
		return errs.New("unable to PEM-encode/write bytes to file", err)
	}

	return nil
}

func writeCerts(certs [][]byte, path string) error {
	file, err := os.Create(path)

	if err != nil {
		return errs.New("unable to open file \"%s\" for writing", path, err)
	}

	defer doubleClose(file)

	for _, cert := range certs {
		if err := writePem(newCertBlock(cert), file); err != nil {
			return err
		}
	}

	if err := file.Close(); err != nil {
		return errs.New("unable to close cert file \"%s\"", path, err)
	}

	return nil
}

func writeKey(key *ecdsa.PrivateKey, path string) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)

	if err != nil {
		return errs.New("unable to open \"%s\" for writing", path, err)
	}

	defer doubleClose(file)

	block, err := keyToBlock(key)
	if err != nil {
		return err
	}

	if err := writePem(block, file); err != nil {
		return err
	}

	if err := file.Close(); err != nil {
		return errs.New("unable to close key filei \"%s\"", path, err)
	}

	return nil
}

func doubleClose(fh io.Closer) {
	err := fh.Close()
	if err == nil || err == os.ErrClosed {
		return
	}
	if perr, ok := err.(*os.PathError); ok && perr.Err == os.ErrClosed {
		return
	}
	log.Printf("Failed to close file: %s\n", err)
}
