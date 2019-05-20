package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertDBOffer(t *testing.T) {
	t.Run("can't create a offer from nil dbx model", func(t *testing.T) {
		offer, err := convertDBOffer(nil)

		require.Nil(t, offer)
		require.NotNil(t, err)
		require.Error(t, err)
	})
}
