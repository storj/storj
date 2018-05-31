package utils

import (
	"os"
	"fmt"
	"path/filepath"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/credentials"
)

var (
  ErrNotExist = errs.Class("")
  ErrNoCreate = errs.Class("creation disabled error")
  ErrNoOverwrite = errs.Class("overwrite disabled error")
  ErrBadHost = errs.Class("bad host error")
  ErrGenerate = errs.Class("tls generation error")
  ErrCredentials = errs.Class("grpc credentials error")
)
func IsNotExist(err error) bool {
	return os.IsNotExist(err) || ErrNotExist.Has(err)
}

type TlsFileOptions struct {
	CertRelPath string
	CertAbsPath string
	// NB: Populate absolute paths from relative paths,
	// 			with respect to pwd via `.EnsureAbsPaths`
	KeyRelPath string
	KeyAbsPath string
	// Create if cert or key nonexistent
	Create bool
	// Overwrite if `create` is true and cert and/or key exist
	Overwrite bool
	// Comma-separated list of hostname(s) (IP or FQDN)
	Hosts     string
}

func (t *TlsFileOptions) EnsureAbsPaths() (_ error){
	if t.CertAbsPath == "" {
		if t.CertRelPath == "" {
			return errs.New("No relative certificate path provided")
		}

		certAbsPath, err := filepath.Abs(t.CertRelPath); if err != nil {
			return errs.New(err.Error())
		}

		t.CertAbsPath = certAbsPath
	}

	if t.KeyAbsPath == "" {
		if t.KeyRelPath == "" {
			return errs.New("No relative key path provided")
		}

		keyAbsPath, err := filepath.Abs(t.KeyRelPath); if err != nil {
			return errs.New(err.Error())
		}

		t.KeyAbsPath = keyAbsPath
	}

	return nil
}

func (t *TlsFileOptions) EnsureExists() (_ error) {
	// Assume cert and key exist
	certMissing, keyMissing := false, false
	errMessage := ""

	err := t.EnsureAbsPaths(); if err != nil {
		return err
	}

	_, err = os.Stat(t.CertAbsPath); if err != nil {
		if !IsNotExist(err) {
			return errs.New(err.Error())
		}

		certMissing = true
	}

	_, err = os.Stat(t.KeyAbsPath); if err != nil {
		if !IsNotExist(err) {
		return errs.New(err.Error())
		}

		keyMissing = true
	}

	if certMissing || keyMissing {
		if t.Create && (t.Overwrite || IsNotExist(err)) {
			return t.generate()
		}

		if certMissing {
			errMessage += fmt.Sprintf("%s and creation disabled\n", err)
		}

		if keyMissing {
			errMessage += fmt.Sprintf("%s and creation disabled\n", err)
		}

		return ErrNotExist.New(errMessage)
	}

	return nil
}

func NewServerTLSFromFile(t *TlsFileOptions) (_ credentials.TransportCredentials,  _ error) {
	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	creds, err := credentials.NewServerTLSFromFile(t.CertAbsPath, t.KeyAbsPath); if err != nil {
		return nil, errs.New(err.Error())
	}

	return creds, nil
}

func NewClientTLSFromFile(t *TlsFileOptions) (_ credentials.TransportCredentials, _ error) {
	if err := t.EnsureExists(); err != nil {
	  return nil, err
  }

	creds, err := credentials.NewClientTLSFromFile(t.CertAbsPath, ""); if err != nil {
	  return nil, ErrCredentials.Wrap(err)
  }

	return creds, nil
}
