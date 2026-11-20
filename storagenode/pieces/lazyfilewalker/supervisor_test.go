// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lazyfilewalker

import (
	"encoding/json"
	"fmt"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/storagenode/pieces/lazyfilewalker/execwrapper"
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

// fakeTrashCleanupCmd emulates the trash-cleanup-filewalker subprocess for
// tests. Like exec.Cmd, each instance is single-use: it carries the mutable
// stdin/stdout/stderr state of one invocation.
type fakeTrashCleanupCmd struct {
	in   io.Reader
	out  io.Writer
	errW io.Writer
	args []string
}

func (cmd *fakeTrashCleanupCmd) SetIn(r io.Reader)     { cmd.in = r }
func (cmd *fakeTrashCleanupCmd) SetOut(w io.Writer)    { cmd.out = w }
func (cmd *fakeTrashCleanupCmd) SetErr(w io.Writer)    { cmd.errW = w }
func (cmd *fakeTrashCleanupCmd) SetArgs(args []string) { cmd.args = args }
func (cmd *fakeTrashCleanupCmd) Start() error          { return nil }
func (cmd *fakeTrashCleanupCmd) Wait() error           { return cmd.Run() }

func (cmd *fakeTrashCleanupCmd) Run() error {
	// decode the request from stdin like the real subprocess does
	var req TrashCleanupRequest
	if err := json.NewDecoder(cmd.in).Decode(&req); err != nil {
		return err
	}
	// give concurrent goroutines a chance to interleave; if a single command
	// instance is shared between runs, this is where the race detector fires
	// on the stream fields above
	time.Sleep(10 * time.Millisecond)
	resp := TrashCleanupResponse{BytesDeleted: 1}
	return json.NewEncoder(cmd.out).Encode(resp)
}

// TestSupervisor_ConcurrentWalkCleanupTrash is a regression test for
// https://github.com/storj/storj/issues/7551. The supervisor used to keep a
// single in-process command instance per filewalker type and hand it to every
// subprocess run, so concurrent runs raced on the command's mutable stream
// state (SetOut writing stdout while another run's command read it). Every
// run must get its own command instance, mirroring the production path where
// each run spawns a new exec.Cmd.
func TestSupervisor_ConcurrentWalkCleanupTrash(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	supervisor := NewSupervisor(log, Config{}, "")

	var factoryCalls atomic.Int64
	supervisor.TestingSetTrashCleanupCmd(func() execwrapper.Command {
		factoryCalls.Add(1)
		return &fakeTrashCleanupCmd{}
	})

	satelliteID := testrand.NodeID()
	dateBefore := time.Now()

	const workers = 8
	var group errgroup.Group
	for i := 0; i < workers; i++ {
		group.Go(func() error {
			_, _, err := supervisor.WalkCleanupTrash(ctx, satelliteID, dateBefore)
			return err
		})
	}
	require.NoError(t, group.Wait())

	// every run must have received its own freshly-built command instance
	require.Equal(t, int64(workers), factoryCalls.Load())
}
