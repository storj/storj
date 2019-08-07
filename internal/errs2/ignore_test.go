package errs2_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/errs2"
)

func TestIsCanceled(t *testing.T) {
	nestedErr := errs.Class("nested error")
	combinedErr := errs.New("combined error")
	parentCtx, cancel := context.WithCancel(context.Background())
	childCtx, _ := context.WithTimeout(parentCtx, 30 * time.Second)

	cancel()

	// context errors
	parentErr := parentCtx.Err()
	childErr := childCtx.Err()

	require.Equal(t, parentErr, context.Canceled)
	require.Equal(t, childErr, context.Canceled)

	require.True(t, errs2.IsCanceled(parentErr))
	require.True(t, errs2.IsCanceled(childErr))

	// nested errors
	nestedParentErr := nestedErr.Wrap(parentErr)
	nestedChildErr := nestedErr.Wrap(childErr)

	require.NotEqual(t, nestedParentErr, context.Canceled)
	require.NotEqual(t, nestedChildErr, context.Canceled)

	require.True(t, errs2.IsCanceled(nestedParentErr))
	require.True(t, errs2.IsCanceled(nestedChildErr))

	// combined errors
	combinedParentErr := errs.Combine(combinedErr, parentErr)
	combinedChildErr := errs.Combine(combinedErr, childErr)

	require.NotEqual(t, combinedParentErr, context.Canceled)
	require.NotEqual(t, combinedChildErr, context.Canceled)

	require.True(t, errs2.IsCanceled(combinedParentErr))
	require.True(t, errs2.IsCanceled(combinedChildErr))
}
