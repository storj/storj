package utils

import (
	"os"
	"fmt"
	"path/filepath"

	"github.com/zeebo/errs"
	"google.golang.org/grpc/credentials"
)

var ErrNotExist = errs.Class("file does not exist")
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
			//return t.generate()
		}

		if certMissing {
			errMessage += fmt.Sprintf("Cert at %s missing and creation disabled\n")
		}

		if keyMissing {
			errMessage += fmt.Sprintf("Key at %s missing and creation disabled\n")
		}

		return errs.New(errMessage)
	}

	return nil
}

func NewServerTLSFromFile(t *TlsFileOptions) (_ credentials.TransportCredentials,  _ error) {
	err := t.EnsureExists(); if err != nil {
		return nil, err
	}

	creds, err := credentials.NewServerTLSFromFile(t.CertAbsPath, t.KeyAbsPath); if err != nil {
		return nil, errs.New(err.Error())
	}

	return creds, nil
}

func NewClientTLSFromFile(t *TlsFileOptions) (_ credentials.TransportCredentials, _ error) {
	t.EnsureExists()
	creds, err := credentials.NewClientTLSFromFile(t.CertAbsPath, "")

	return creds, errs.New(err.Error())
}
