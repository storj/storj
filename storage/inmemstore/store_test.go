package inmemstore

import (
	"testing"

	"storj.io/storj/storage"
)

func TestCommon(t *testing.T) {
	storage.RunTests(t, storage.NewTestLogger(t, New()))
}
