package utils_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/utils"
)

func TestCollectSingleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- returnError()
	}()

	err := utils.CollectErrors(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error")
}

func TestCollecMultipleError(t *testing.T) {
	errchan := make(chan error)
	defer close(errchan)

	go func() {
		errchan <- returnError()
		errchan <- returnError()
		errchan <- returnError()
	}()

	err := utils.CollectErrors(errchan, 1*time.Second)
	assert.Error(t, err)
	assert.Equal(t, err.Error(), "error\nerror\nerror")
}

func returnError() error {
	return errs.New("error")
}
