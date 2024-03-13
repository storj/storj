// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package nodeoperator_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/private/nodeoperator"
)

func TestWalletFeaturesValidation(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		var validation nodeoperator.WalletFeaturesValidation

		err := validation.Validate([]string{})
		require.NoError(t, err)

		err = validation.Validate(nil)
		require.NoError(t, err)
	})

	t.Run("exceeds list limit", func(t *testing.T) {
		features := []string{
			"feature1",
			"feature2",
			"feature3",
			"feature4",
			"feature5",
			"feature6",
		}

		validation := nodeoperator.WalletFeaturesValidation{
			MaxListLength:    5,
			MaxFeatureLength: 20,
		}

		err := validation.Validate(features)
		require.Error(t, err)
	})

	t.Run("exceeds feature length", func(t *testing.T) {
		features := []string{
			"feature1",
			"feature2",
			"feature3",
			"feature4",
			"feature5",
			"invalidFeature",
		}

		validation := nodeoperator.WalletFeaturesValidation{
			MaxListLength:    6,
			MaxFeatureLength: 10,
		}

		err := validation.Validate(features)
		require.Error(t, err)
	})

	t.Run("contains reserved characters", func(t *testing.T) {
		features := []string{
			"feature1",
			"feature2",
			"feature3",
			"feature4",
			"feature5",
			"feature|",
		}

		validation := nodeoperator.WalletFeaturesValidation{
			MaxListLength:      6,
			MaxFeatureLength:   10,
			ReservedCharacters: []rune{'|'},
		}

		err := validation.Validate(features)
		require.Error(t, err)
	})
}
