// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleql

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/satellite/console"
)

func TestHandleError(t *testing.T) {
	err := console.ErrConsoleInternal.New("a")
	handledError := HandleError(err)

	assert.Equal(t, handledError.Error(), internalErrDetailedMsg)
}
