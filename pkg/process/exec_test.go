package process

import (
	"fmt"
	"testing"
)

type mockService struct {
	Called int
}

var (
	getCases = []struct {
		testID              string
		expectedTimesCalled int
		expectedResponse    error
	}{
		// test cases
		{
			testID:              "valid process",
			expectedTimesCalled: 1,
			expectedResponse:    nil,
		},
		{
			testID:              "multiple processes",
			expectedTimesCalled: 1,
			expectedResponse:    nil,
		},
		{
			testID:              "error process",
			expectedTimesCalled: 1,
			expectedResponse:    nil,
		},
	}
)

func TestMain(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			fmt.Printf("running test %s", c.testID)
		})
	}
}
