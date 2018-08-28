package teststore

import (
	"testing"

	"storj.io/storj/storage"
)

func TestCommon(t *testing.T) {
	storage.RunTests(t, New())
}
