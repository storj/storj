package utils

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/credentials"
	"crypto/ecdsa"
	"crypto/tls"
)

var (
	ErrNotExist = errs.Class("")
	// TODO: use or not
	// ErrNoCreate    = errs.Class("creation disabled error")
	// ErrNoOverwrite = errs.Class("overwrite disabled error")
	ErrBadHost     = errs.Class("bad host error")
	ErrGenerate    = errs.Class("tls generation error")
	ErrCredentials = errs.Class("grpc credentials error")
)

func IsNotExist(err error) bool {
	return os.IsNotExist(err) || ErrNotExist.Has(err)
}

type TLSFileOptions struct {
	CertRelPath string
	CertAbsPath string
	Certificate *tls.Certificate
	// NB: Populate absolute paths from relative paths,
	// 			with respect to pwd via `.EnsureAbsPaths`
	KeyRelPath string
	KeyAbsPath string
	Key        *ecdsa.PrivateKey
	// Create if cert or key nonexistent
	Create bool
	// Overwrite if `create` is true and cert and/or key exist
	Overwrite bool
	// Comma-separated list of hostname(s) (IP or FQDN)
	Hosts string
	// If true, key is not required or checked
	Client bool
}

// func NewTLSFileOptions(certPath, keyPath string, client, overwrite bool, hosts string) (_ *TLSFileOptions, _ error) {
// 	t := TLSFileOptions{
// 		CertRelPath: certPath,
// 		KeyRelPath:  keyPath,
// 		Client:      client,
// 		Overwrite:   overwrite,
// 		Hosts:       hosts,
// 	}
//
// 	if err := t.EnsureAbsPaths(); err != nil {
// 		return nil, err
// 	}
//
// }

func (t *TLSFileOptions) EnsureAbsPaths() (_ error) {
	if t.CertAbsPath == "" {
		if t.CertRelPath == "" {
			return errs.New("No relative certificate path provided")
		}

		certAbsPath, err := filepath.Abs(t.CertRelPath)
		if err != nil {
			return errs.New(err.Error())
		}

		t.CertAbsPath = certAbsPath
	}

	if t.KeyAbsPath == "" {
		if t.KeyRelPath == "" {
			return errs.New("No relative key path provided")
		}

		keyAbsPath, err := filepath.Abs(t.KeyRelPath)
		if err != nil {
			return errs.New(err.Error())
		}

		t.KeyAbsPath = keyAbsPath
	}

	return nil
}

func (t *TLSFileOptions) EnsureExists() (_ error) {
	// Assume cert and key exist
	certMissing, keyMissing := false, false
	errMessage := ""

	if err := t.EnsureAbsPaths(); err != nil {
		return err
	}

	if _, err := os.Stat(t.CertAbsPath); err != nil {
		if !IsNotExist(err) {
			return errs.New(err.Error())
		}

		errMessage += fmt.Sprintf("%s and creation disabled\n", err)
		certMissing = true
	}

	if _, err := os.Stat(t.KeyAbsPath); err != nil {
		if !IsNotExist(err) {
			return errs.New(err.Error())
		}

		errMessage += fmt.Sprintf("%s and creation disabled\n", err)
		keyMissing = true
	}

	// NB: even when `overwrite` is false, this WILL overwrite
	//      a key if the cert is missing (vice versa)
	if t.Create && (t.Overwrite || certMissing || keyMissing) {
		var (
			cert *tls.Certificate
			err error
		)

		if t.Client {
			cert, err = t.generateClientTls()
		} else {
			cert, err = t.generateServerTls()
		}

		if err != nil {
			return err
		}

		t.Certificate = cert
		return nil
	}

	if certMissing || keyMissing {
		return ErrNotExist.New(errMessage)
	}

	return nil
}

func NewServerTLSFromFile(t *TLSFileOptions) (_ credentials.TransportCredentials, _ error) {
	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	creds, err := credentials.NewServerTLSFromFile(t.CertAbsPath, t.KeyAbsPath)
	if err != nil {
		return nil, errs.New(err.Error())
	}

	return creds, nil
}

func NewClientTLSFromFile(t *TLSFileOptions) (_ credentials.TransportCredentials, _ error) {
	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	creds, err := credentials.NewClientTLSFromFile(t.CertAbsPath, "")
	if err != nil {
		return nil, ErrCredentials.Wrap(err)
	}

	return creds, nil
}
