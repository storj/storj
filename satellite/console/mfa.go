// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package console

import (
	"context"
	"crypto/rand"
	"math/big"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/zeebo/errs"

	"storj.io/storj/private/post"
)

const (
	// MFARecoveryCodeCount specifies how many MFA recovery codes to generate.
	MFARecoveryCodeCount = 10
)

// Error messages.
const (
	mfaPasscodeInvalidErrMsg    = "The MFA passcode is not valid or has expired"
	mfaRequiredErrMsg           = "A MFA passcode or recovery code is required"
	mfaRecoveryInvalidErrMsg    = "The MFA recovery code is not valid or has been previously used"
	mfaRecoveryGenerationErrMsg = "MFA recovery codes cannot be generated while MFA is disabled."
	mfaConflictErrMsg           = "Expected either passcode or recovery code, but got both"
)

var (
	// ErrMFAMissing is error type that occurs when a request is incomplete
	// due to missing MFA passcode or recovery code.
	ErrMFAMissing = errs.Class("MFA credentials missing")

	// ErrMFAConflict is error type that occurs when both a passcode and recovery code are given.
	ErrMFAConflict = errs.Class("MFA conflict")

	// ErrMFAEnabled is error type that occurs when user tries to enable MFA when it's already enabled.
	ErrMFAEnabled = errs.Class("MFA already enabled")

	// ErrMFARecoveryCode is error type that represents usage of invalid MFA recovery code.
	ErrMFARecoveryCode = errs.Class("MFA recovery code")

	// ErrMFAPasscode is error type that represents usage of invalid MFA passcode.
	ErrMFAPasscode = errs.Class("MFA passcode")
)

// NewMFAValidationOpts returns the options used to validate TOTP passcodes.
// These settings are also used to generate MFA secret keys for use in testing.
func NewMFAValidationOpts() totp.ValidateOpts {
	return totp.ValidateOpts{
		Period:    30,
		Skew:      1,
		Digits:    6,
		Algorithm: otp.AlgorithmSHA1,
	}
}

// ValidateMFAPasscode returns whether the TOTP passcode is valid for the secret key at the given time.
func ValidateMFAPasscode(passcode string, secretKey string, t time.Time) (bool, error) {
	valid, err := totp.ValidateCustom(passcode, secretKey, t, NewMFAValidationOpts())
	return valid, Error.Wrap(err)
}

// NewMFAPasscode derives a TOTP passcode from a secret key using a timestamp.
func NewMFAPasscode(secretKey string, t time.Time) (string, error) {
	code, err := totp.GenerateCodeCustom(secretKey, t, NewMFAValidationOpts())
	return code, Error.Wrap(err)
}

// NewMFASecretKey generates a new TOTP secret key.
func NewMFASecretKey() (string, error) {
	opts := NewMFAValidationOpts()
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      " ",
		AccountName: " ",
		Period:      opts.Period,
		Digits:      otp.DigitsSix,
		Algorithm:   opts.Algorithm,
	})
	if err != nil {
		return "", Error.Wrap(err)
	}
	return key.Secret(), nil
}

// EnableUserMFA enables multi-factor authentication for the user if the given secret key and password are valid.
func (s *Service) EnableUserMFA(ctx context.Context, passcode string, t time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "enable MFA")
	if err != nil {
		return Error.Wrap(err)
	}

	if user.MFAEnabled {
		return ErrMFAEnabled.New("")
	}

	valid, err := ValidateMFAPasscode(passcode, user.MFASecretKey, t)
	if err != nil {
		return ErrValidation.Wrap(ErrMFAPasscode.Wrap(err))
	}
	if !valid {
		return ErrValidation.Wrap(ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg))
	}

	user.MFAEnabled = true
	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		MFAEnabled: &user.MFAEnabled,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: user.FullName}},
		&MFAActivatedEmail{},
	)

	return nil
}

// DisableUserMFA disables multi-factor authentication for the user if the given secret key and password are valid.
func (s *Service) DisableUserMFA(ctx context.Context, passcode string, t time.Time, recoveryCode string) (err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "disable MFA")
	if err != nil {
		return Error.Wrap(err)
	}

	if !user.MFAEnabled {
		return nil
	}

	if recoveryCode != "" && passcode != "" {
		return ErrMFAConflict.New(mfaConflictErrMsg)
	}

	if recoveryCode != "" {
		found := false
		for _, code := range user.MFARecoveryCodes {
			if code == recoveryCode {
				found = true
				break
			}
		}
		if !found {
			return ErrMFARecoveryCode.New(mfaRecoveryInvalidErrMsg)
		}
	} else if passcode != "" {
		valid, err := ValidateMFAPasscode(passcode, user.MFASecretKey, t)
		if err != nil {
			return ErrValidation.Wrap(ErrMFAPasscode.Wrap(err))
		}
		if !valid {
			return ErrValidation.Wrap(ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg))
		}
	} else {
		return ErrMFAMissing.New(mfaRequiredErrMsg)
	}

	user.MFAEnabled = false
	user.MFASecretKey = ""
	user.MFARecoveryCodes = nil

	secretKeyPtr := &user.MFASecretKey

	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		MFAEnabled:       &user.MFAEnabled,
		MFASecretKey:     &secretKeyPtr,
		MFARecoveryCodes: &user.MFARecoveryCodes,
	})
	if err != nil {
		return Error.Wrap(err)
	}

	s.mailService.SendRenderedAsync(
		ctx,
		[]post.Address{{Address: user.Email, Name: user.FullName}},
		&MFADisabledEmail{},
	)

	return nil
}

// NewMFARecoveryCode returns a randomly generated MFA recovery code.
// Recovery codes are uppercase and alphanumeric. They are of the form XXXX-XXXX-XXXX.
func NewMFARecoveryCode() (string, error) {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, 14)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < 14; i++ {
		if (i+1)%5 == 0 {
			b[i] = '-'
		} else {
			num, err := rand.Int(rand.Reader, max)
			if err != nil {
				return "", err
			}
			b[i] = chars[num.Int64()]
		}
	}
	return string(b), nil
}

// ResetMFASecretKey creates a new TOTP secret key for the user.
func (s *Service) ResetMFASecretKey(ctx context.Context) (key string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "reset MFA secret key")
	if err != nil {
		return "", Error.Wrap(err)
	}

	if user.MFAEnabled {
		return "", ErrMFAEnabled.New("")
	}

	key, err = NewMFASecretKey()
	if err != nil {
		return "", Error.Wrap(err)
	}

	user.MFASecretKey = key
	mfaSecretKeyPtr := &user.MFASecretKey

	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		MFASecretKey: &mfaSecretKeyPtr,
	})
	if err != nil {
		return "", Error.Wrap(err)
	}

	return key, nil
}

// ResetMFARecoveryCodes creates a new set of MFA recovery codes for the user.
func (s *Service) ResetMFARecoveryCodes(ctx context.Context, requireCode bool, passcode string, recoveryCode string) (codes []string, err error) {
	defer mon.Task()(&ctx)(&err)

	user, err := s.getUserAndAuditLog(ctx, "reset MFA recovery codes")
	if err != nil {
		return nil, Error.Wrap(err)
	}

	if !user.MFAEnabled {
		return nil, ErrUnauthorized.New(mfaRecoveryGenerationErrMsg)
	}

	if requireCode {
		t := time.Now()
		if recoveryCode != "" && passcode != "" {
			return nil, ErrMFAConflict.New(mfaConflictErrMsg)
		}

		if recoveryCode != "" {
			found := false
			for _, code := range user.MFARecoveryCodes {
				if code == recoveryCode {
					found = true
					break
				}
			}
			if !found {
				return nil, ErrMFARecoveryCode.New(mfaRecoveryInvalidErrMsg)
			}
		} else if passcode != "" {
			valid, err := ValidateMFAPasscode(passcode, user.MFASecretKey, t)
			if err != nil {
				return nil, ErrValidation.Wrap(ErrMFAPasscode.Wrap(err))
			}
			if !valid {
				return nil, ErrValidation.Wrap(ErrMFAPasscode.New(mfaPasscodeInvalidErrMsg))
			}
		} else {
			return nil, ErrMFAMissing.New(mfaRequiredErrMsg)
		}
	}

	codes = make([]string, MFARecoveryCodeCount)
	for i := 0; i < MFARecoveryCodeCount; i++ {
		code, err := NewMFARecoveryCode()
		if err != nil {
			return nil, Error.Wrap(err)
		}
		codes[i] = code
	}

	err = s.store.Users().Update(ctx, user.ID, UpdateUserRequest{
		MFARecoveryCodes: &codes,
	})
	if err != nil {
		return nil, Error.Wrap(err)
	}

	return codes, nil
}
