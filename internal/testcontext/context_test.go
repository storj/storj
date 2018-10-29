package testcontext_test

import (
	"testing"
	"time"

	"storj.io/storj/internal/testcontext"
)

func TestBasic(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ctx.Go(func() error {
		time.Sleep(time.Millisecond)
		return nil
	})

	t.Log(ctx.Dir("a", "b", "c"))
	t.Log(ctx.File("a", "w", "c.txt"))
}
