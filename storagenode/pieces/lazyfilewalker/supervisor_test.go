// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/common/storj"
)

func TestTrashHandler_Write(t *testing.T) {

	t.Run("basic test", func(t *testing.T) {
		pieceIDs := []storj.PieceID{storj.NewPieceID(), storj.NewPieceID(), storj.NewPieceID()}

		numTrashed := 0
		trashFunc := func(pieceID storj.PieceID) error {
			numTrashed++
			require.Contains(t, pieceIDs, pieceID)
			return nil
		}

		expectedFinalResponse := GCFilewalkerResponse{
			// final response will have all trash pieceIDS, assuming there are 5 pieces
			// in total and 2 were not trash pieces.
			PieceIDs:           pieceIDs,
			PiecesCount:        5,
			PiecesSkippedCount: 2,
			Completed:          true,
		}

		outputs := []GCFilewalkerResponse{
			{
				PieceIDs: []storj.PieceID{pieceIDs[0]},
			},
			{
				PieceIDs: []storj.PieceID{pieceIDs[1]},
			},
			{
				PieceIDs: []storj.PieceID{pieceIDs[2]},
			},
			expectedFinalResponse,
		}

		trashHandler := NewTrashHandler(zaptest.NewLogger(t), trashFunc)

		for _, output := range outputs {
			err := json.NewEncoder(trashHandler).Encode(output)
			require.NoError(t, err)
		}

		var resp GCFilewalkerResponse
		err := trashHandler.Decode(&resp)
		require.NoError(t, err)

		// check that the final response is as expected
		require.Equal(t, expectedFinalResponse, resp)

		// check that the trashHandler processed all the trash pieces
		require.Equal(t, len(pieceIDs), numTrashed)
	})

	// this test simulates the case where the output is truncated
	// and the trashHandler receives the output in multiple chunks
	// and processes the trash pieces correctly
	t.Run("truncated output", func(t *testing.T) {
		pieceIDs := []storj.PieceID{storj.NewPieceID(), storj.NewPieceID(), storj.NewPieceID(), storj.NewPieceID()}

		numTrashed := 0
		trashFunc := func(pieceID storj.PieceID) error {
			numTrashed++
			require.Contains(t, pieceIDs, pieceID)
			return nil
		}

		trashHandler := NewTrashHandler(zaptest.NewLogger(t), trashFunc)

		// The string slice below is a concatenation of multiple JSON outputs:
		// {"pieceIDs":["<pieceID0>"]}\n
		// {"pieceIDs":["<pieceID1>"]}\n{pieceIDs"
		// :["<pieceID2>"]}\n{"pieceIDs":["<pieceID3>"]}\n
		// {"pieceIDs":["<pieceID0>",
		// "<pieceID1>",
		// "<pieceID2>",
		// ",<pieceID3>"], "piecesCount": 4, "piecesSkippedCount": 0, "completed": true}\n
		outputs := []string{
			fmt.Sprintf("{\"pieceIDs\":[%q]}\n", pieceIDs[0]),
			fmt.Sprintf("{\"pieceIDs\":[%q]}\n{\"pieceIDs\"", pieceIDs[1]),
			fmt.Sprintf(":[%q]}\n{\"pieceIDs\":[%q]}\n", pieceIDs[2], pieceIDs[3]),
			fmt.Sprintf("{\"pieceIDs\":[%q,", pieceIDs[0]),
			fmt.Sprintf("%q,", pieceIDs[1]),
			fmt.Sprintf("%q", pieceIDs[2]),
			fmt.Sprintf(",%q], \"piecesCount\": 4, \"piecesSkippedCount\": 0, \"completed\": true}\n", pieceIDs[3]),
		}

		for _, output := range outputs {
			_, err := trashHandler.Write([]byte(output))
			require.NoError(t, err)
		}

		expectedFinalResponse := GCFilewalkerResponse{
			PieceIDs:           pieceIDs,
			PiecesCount:        4,
			PiecesSkippedCount: 0,
			Completed:          true,
		}

		var resp GCFilewalkerResponse
		err := trashHandler.Decode(&resp)
		require.NoError(t, err)

		// check that the final response is as expected
		require.Equal(t, expectedFinalResponse, resp)

		// check that the trashHandler processed all the trash pieces
		require.Equal(t, len(pieceIDs), numTrashed)
	})
}
