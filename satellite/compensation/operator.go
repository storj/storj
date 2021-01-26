// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package compensation

import "strings"

// WalletFeatures represents wallet features list.
type WalletFeatures []string

// DecodeWalletFeatures decodes wallet features list string separated by "|".
func DecodeWalletFeatures(s string) (WalletFeatures, error) {
	if s == "" {
		return nil, nil
	}
	return strings.Split(s, "|"), nil
}

// String outputs .
func (features WalletFeatures) String() string {
	return strings.Join(features, "|")
}

// UnmarshalCSV reads the WalletFeatures in CSV form.
func (features *WalletFeatures) UnmarshalCSV(s string) error {
	v, err := DecodeWalletFeatures(s)
	if err != nil {
		return err
	}
	*features = v
	return nil
}

// MarshalCSV returns the CSV form of the WalletFeatures.
func (features WalletFeatures) MarshalCSV() (string, error) {
	return features.String(), nil
}
