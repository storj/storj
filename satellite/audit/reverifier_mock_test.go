package audit_test

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"

	"storj.io/common/uuid"
	"storj.io/storj/satellite/audit"
	"storj.io/storj/satellite/metabase"
	"storj.io/storj/satellite/overlay"
)

type mockVerifier struct{}

func (verifier *mockVerifier) DoReverifyPiece(ctx context.Context, logger *zap.Logger, locator *audit.PieceLocator) (audit.Outcome, overlay.ReputationStatus, error) {
	return audit.OutcomeSuccess, overlay.ReputationStatus{}, nil
}

type mockDB struct{}

func (db *mockDB) Next(ctx context.Context, limit int) ([]audit.ReverificationJob, error) {
	return []audit.ReverificationJob{}, nil
}

func (db *mockDB) Done(ctx context.Context, job audit.ReverificationJob, outcome audit.Outcome) error {
	return nil
}

func TestReverifier_ReverifyPiece(t *testing.T) {
	ctx := context.Background()
	log := zap.NewNop()

	verifier := &mockVerifier{}
	db := &mockDB{}

	reverifier := audit.NewReverifier(log, verifier, db, audit.Config{ReverificationRetryInterval: time.Hour})

	var locator = &audit.PieceLocator{
		StreamID: uuid.New(),
		Position: metabase.SegmentPosition{},
		NodeID:   [32]byte{},
 		PieceNum: 0,
 	}
	outcome, reputation, err := reverifier.ReverifyPiece(ctx, log, locator)
	if err != nil {
		t.Fatal("expected err to be nil but got", err)
	}
	if outcome != audit.OutcomeSuccess {
		t.Fatal("expected outcome to be", audit.OutcomeSuccess, "but got", outcome)
	}
	if reputation != (overlay.ReputationStatus{}) {
		t.Fatal("expected reputation to be", overlay.ReputationStatus{}, "but got", reputation)
	}
}
