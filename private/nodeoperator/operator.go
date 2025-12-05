// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeoperator

import (
	"strings"

	"github.com/zeebo/errs"
)

// DefaultWalletFeaturesValidation contains default wallet features list validation config.
var DefaultWalletFeaturesValidation = WalletFeaturesValidation{
	MaxListLength:      5,
	MaxFeatureLength:   15,
	ReservedCharacters: []rune{',', '|'},
}

// WalletFeatureValidationError wallet feature validation errors class.
var WalletFeatureValidationError = errs.Class("wallet feature validation")

// WalletFeaturesValidation contains config for wallet feature validation.
type WalletFeaturesValidation struct {
	MaxListLength      int
	MaxFeatureLength   int
	ReservedCharacters []rune
}

// Validate validates wallet features list.
func (validation *WalletFeaturesValidation) Validate(features []string) error {
	var errGroup errs.Group

	if len(features) == 0 {
		return nil
	}

	if len(features) > validation.MaxListLength {
		errGroup.Add(
			errs.New("features list exceeds maximum length, %d > %d", len(features), validation.MaxListLength))
	}

	for _, feature := range features {
		if len(feature) > validation.MaxFeatureLength {
			errGroup.Add(
				errs.New("feature %q exceeds maximum length, %d > %d", feature, len(feature), validation.MaxFeatureLength))
		}

		for _, reserved := range validation.ReservedCharacters {
			if i := strings.IndexRune(feature, reserved); i >= 0 {
				errGroup.Add(errs.New("feature %q contains reserved character '%c' at pos %d", feature, reserved, i))
			}
		}
	}

	return WalletFeatureValidationError.Wrap(errGroup.Err())
}
