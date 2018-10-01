package put

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestValidateArgs(t *testing.T) {
	cases := []struct {
		testName string
		testFunc func(*testing.T)
	} {
		{
			"Valid args length",
			func(t *testing.T) {
				err := validateArgs(nil, []string{"bucket", "object"})
				assert.NoError(t, err)
			},
		},
		{
			"Too many args",
			func(t *testing.T) {
				err := validateArgs(nil, []string{"bucket", "object", "thirdarg"})
				assert.Error(t, err)
				assert.Equal(t, NewInvalidArgsError(3).Error(), err.Error())
			},
		},
		{
			"Not enough args",
			func(t *testing.T) {
				err := validateArgs(nil, []string{"bucket"})
				assert.Error(t, err)
				assert.Equal(t, NewInvalidArgsError(1).Error(), err.Error())
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, c.testFunc)
	}
}
