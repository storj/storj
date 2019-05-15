package satellitedb

import (
	"testing"

	"github.com/stretchr/testify/require"

	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

func TestConvertDBOffer(t *testing.T) {
	t.Run("can't create a offer from nil dbx model", func(t *testing.T) {
		offer, err := convertDBOffer(nil)

		require.Nil(t, offer)
		require.NotNil(t, err)
		require.Error(t, err)
	})

	t.Run("can't create a offer from dbx model with invalid id", func(t *testing.T) {
		dbxOffer := dbx.Offer{
			Id: []byte("test"),
		}
		offer, err := convertDBOffer(&dbxOffer)

		require.Nil(t, offer)
		require.Error(t, err)
	})
}
