// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

func TestSignedTags(t *testing.T) {
	signer, err := storj.NodeIDFromString("12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4")
	require.NoError(t, err)

	t.Run("single injection", func(t *testing.T) {
		s := SignedTags{}

		// created with `tag-signer sign --node-id 1AujhDBDBhfYUhEH8cpFMymPu5yLYkQBCfFYbdSN5AVL5Bu6W9 --identity-dir satellite-api/0 soc=true`.
		err := s.Set("CqIBCjUKIBaACbi52g7o8pQaX+hkw6TJNDHJ0UVqSLQtibPHoqkAEgsKA3NvYxIEdHJ1ZRjVqoqlBhog/+bJH0ducPXXHjw4eOBZLzO4LwHd+nZvQGIFy66jcwAiRzBFAiEAhrP90d2VxTHnWFDTzOv7Xd5MlvPon4lMgE9QotzLMmYCIAtXUbCrIVZEiphblFuaDDftJY0XTm/n64wZthuB8SJx")
		require.NoError(t, err)

		pbTags := pb.SignedNodeTagSets(s).Tags
		require.Len(t, pbTags, 1)
		require.Equal(t, signer.Bytes(), pbTags[0].SignerNodeId)

	})

	t.Run("coma separated", func(t *testing.T) {
		s := SignedTags{}

		a := "CqIBCjUKIBaACbi52g7o8pQaX+hkw6TJNDHJ0UVqSLQtibPHoqkAEgsKA3NvYxIEdHJ1ZRjVqoqlBhog/+bJH0ducPXXHjw4eOBZLzO4LwHd+nZvQGIFy66jcwAiRzBFAiEAhrP90d2VxTHnWFDTzOv7Xd5MlvPon4lMgE9QotzLMmYCIAtXUbCrIVZEiphblFuaDDftJY0XTm/n64wZthuB8SJx"
		b := "CqEBCjQKIBaACbi52g7o8pQaX+hkw6TJNDHJ0UVqSLQtibPHoqkAEgoKA2ZvbxIDYmFyGMesiqUGGiD/5skfR25w9dcePDh44FkvM7gvAd36dm9AYgXLrqNzACJHMEUCIDFJMkpi3z3qPxvLch7Ie7afpP7Ab8+wsayzCGo0WMaBAiEAgCoWfSUhXNeFx2FPrlAv0ed5jW/DH+7TjDdeiqwA04g="
		err := s.Set(a + "," + b)
		require.NoError(t, err)

		pbTags := pb.SignedNodeTagSets(s).Tags
		require.Len(t, pbTags, 2)
		require.Equal(t, signer.Bytes(), pbTags[0].SignerNodeId)

	})
}
