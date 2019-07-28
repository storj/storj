// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/storj/internal/errs2"
)

func TestGroup(t *testing.T) {
	group := errs2.Group{}
	group.Go(func() error {
		return fmt.Errorf("first")
	})
	group.Go(func() error {
		return nil
	})
	group.Go(func() error {
		return fmt.Errorf("second")
	})
	group.Go(func() error {
		return fmt.Errorf("third")
	})

	allErrors := group.Wait()
	require.Len(t, allErrors, 3)
}
