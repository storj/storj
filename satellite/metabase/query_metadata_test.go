package metabase

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSplitToJSONLeaves(t *testing.T) {
	input := `{
		"key1": "value1",
		"key2": {
			"key3": "value3",
			"key4": [
				1,
				2,
				{
					"key5": "value5",
					"key6": "value6"
				}
			]
		}
	}`

	expected := []string{
		`{"key1":"value1"}`,
		`{"key2":{"key3":"value3"}}`,
		`{"key2":{"key4":[1]}}`,
		`{"key2":{"key4":[2]}}`,
		`{"key2":{"key4":[{"key5":"value5"}]}}`,
		`{"key2":{"key4":[{"key6":"value6"}]}}`,
	}

	actual, err := splitToJSONLeaves(input)
	require.NoError(t, err)
	require.ElementsMatch(t, expected, actual)
}
