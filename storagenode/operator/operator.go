// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package operator

import (
	"errors"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"storj.io/storj/private/nodeoperator"
)

// Config defines properties related to storage node operator metadata.
type Config struct {
	Email          string         `user:"true" help:"operator email address" default:""`
	Wallet         string         `user:"true" help:"operator wallet address" default:""`
	WalletFeatures WalletFeatures `user:"true" help:"operator wallet features" default:""`
}

// Verify verifies whether operator config is valid.
func (c Config) Verify(log *zap.Logger) error {
	if err := isOperatorEmailValid(log, c.Email); err != nil {
		return err
	}
	if err := isOperatorWalletValid(log, c.Wallet); err != nil {
		return err
	}
	if err := isOperatorWalletFeaturesValid(log, c.WalletFeatures); err != nil {
		return err
	}
	return nil
}

func isOperatorEmailValid(log *zap.Logger, email string) error {
	if email == "" {
		log.Warn("Operator email address isn't specified.")
	} else {
		log.Info("Operator email", zap.String("Address", email))
	}
	return nil
}

func isOperatorWalletValid(log *zap.Logger, wallet string) error {
	if wallet == "" {
		return errors.New("operator wallet address isn't specified")
	}
	r := regexp.MustCompile("^0x[a-fA-F0-9]{40}$")
	if match := r.MatchString(wallet); !match {
		return errors.New("operator wallet address isn't valid")
	}

	log.Info("Operator wallet", zap.String("Address", wallet))
	return nil
}

// isOperatorWalletFeaturesValid checks if wallet features list does not exceed length limits.
func isOperatorWalletFeaturesValid(log *zap.Logger, features WalletFeatures) error {
	return nodeoperator.DefaultWalletFeaturesValidation.Validate(features)
}

// ensure WalletFeatures implements pflag.Value.
var _ pflag.Value = (*WalletFeatures)(nil)

// WalletFeatures payout opt-in wallet features list.
type WalletFeatures []string

// String returns the comma separated list of wallet features.
func (features WalletFeatures) String() string {
	return strings.Join(features, ",")
}

// Set implements pflag.Value by parsing a comma separated list of wallet features.
func (features *WalletFeatures) Set(value string) error {
	if value != "" {
		*features = strings.Split(value, ",")
	}
	return nil
}

// Type returns the type of the pflag.Value.
func (features WalletFeatures) Type() string {
	return "wallet-features"
}
