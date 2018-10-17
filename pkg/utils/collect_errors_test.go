package utils_test

import (
	"testing"

	"github.com/zeebo/errs"
)

var (
	testCases = []struct {
		testID        string
		errorslice    []error
		expectedError error
	}{
		{
			testID:        "valid collection",
			errorslice:    []error{errs.New("collecterror")},
			expectedError: errs.New("collecterror"),
		},
	}
)

func TestCollectErrors(t *testing.T) {

	for _, c := range testCases {
		t.Run(c.testID, func(t *testing.T) {
			t.Logf("starting test case: %s\n %+v\n %+v\n", c.testID, c.errorslice, c.expectedError)
		})
	}
}
