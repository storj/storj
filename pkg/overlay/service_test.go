package overlay

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/internal/test"
)

func TestProcess(t *testing.T) {
	done := test.EnsureRedis(t)
	defer done()

	o := Service{}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := o.Process(ctx)
	assert.NoError(t, err)
}
