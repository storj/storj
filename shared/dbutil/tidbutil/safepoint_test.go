// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/tikv/pd/client/clients/gc"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/common/testcontext"
)

type fakePD struct {
	mu sync.Mutex

	physical int64
	logical  int64

	barrierErr   error // returned by SetGCBarrier when set
	barriers     map[string]uint64
	barrierSets  int // counts SetGCBarrier calls, so refreshes are observable
	legacy       map[string]uint64
	minSafepoint uint64
	closed       bool
}

// barrierSetCount reports how many times SetGCBarrier has been called.
func (f *fakePD) barrierSetCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.barrierSets
}

func newFakePD(physical, logical int64) *fakePD {
	return &fakePD{
		physical: physical,
		logical:  logical,
		barriers: map[string]uint64{},
		legacy:   map[string]uint64{},
	}
}

func (f *fakePD) GetTS(ctx context.Context) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.physical, f.logical, nil
}

func (f *fakePD) GetGCStatesClient(keyspaceID uint32) gc.GCStatesClient {
	return &fakeGCStates{pd: f}
}

func (f *fakePD) UpdateServiceGCSafePoint(ctx context.Context, serviceID string, ttl int64, safePoint uint64) (uint64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ttl <= 0 {
		delete(f.legacy, serviceID)
	} else {
		f.legacy[serviceID] = safePoint
	}
	return f.minSafepoint, nil
}

func (f *fakePD) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
}

type fakeGCStates struct {
	gc.GCStatesClient // panics on unimplemented methods

	pd *fakePD
}

func (f *fakeGCStates) SetGCBarrier(ctx context.Context, barrierID string, barrierTS uint64, ttl time.Duration) (*gc.GCBarrierInfo, error) {
	f.pd.mu.Lock()
	defer f.pd.mu.Unlock()
	f.pd.barrierSets++
	if f.pd.barrierErr != nil {
		return nil, f.pd.barrierErr
	}
	f.pd.barriers[barrierID] = barrierTS
	return &gc.GCBarrierInfo{BarrierID: barrierID, BarrierTS: barrierTS, TTL: ttl}, nil
}

func (f *fakeGCStates) DeleteGCBarrier(ctx context.Context, barrierID string) (*gc.GCBarrierInfo, error) {
	f.pd.mu.Lock()
	defer f.pd.mu.Unlock()
	delete(f.pd.barriers, barrierID)
	return nil, nil
}

func TestHolder_BarrierAPI(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	physical := time.Now().UnixMilli()
	fake := newFakePD(physical, 42)

	holder, err := hold(ctx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	// The logical part (42) must not reach the barrier: callers read AS OF
	// ReadTime, which TiDB resolves to (unix_ms << 18), so a barrier above that
	// would leave the read below the GC safe point once the barrier binds.
	expectedTSO := uint64(physical) << tsoPhysicalShiftBits
	fake.mu.Lock()
	if got := fake.barriers[holder.ServiceID()]; got != expectedTSO {
		t.Fatalf("barrier at %d, expected %d", got, expectedTSO)
	}
	fake.mu.Unlock()

	// The invariant the above protects: the barrier is exactly the timestamp
	// callers read at, not merely near it.
	readTSO := uint64(holder.ReadTime().UnixMilli()) << tsoPhysicalShiftBits
	if readTSO != expectedTSO {
		t.Fatalf("ReadTime resolves to tso %d but the barrier is at %d; "+
			"a read below the barrier is rejected once the barrier binds", readTSO, expectedTSO)
	}

	if !holder.ReadTime().Equal(time.UnixMilli(physical).UTC()) {
		t.Fatalf("read time %v does not match physical %d", holder.ReadTime(), physical)
	}
	if holder.ReadTime().After(time.UnixMilli(physical)) {
		t.Fatal("read time must not exceed the protected timestamp")
	}

	if err := holder.Release(ctx); err != nil {
		t.Fatal(err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, ok := fake.barriers[holder.ServiceID()]; ok {
		t.Fatal("barrier was not removed on release")
	}
	if !fake.closed {
		t.Fatal("client was not closed on release")
	}
}

func TestHolder_LegacyFallback(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	physical := time.Now().UnixMilli()
	fake := newFakePD(physical, 0)
	fake.barrierErr = status.Error(codes.Unimplemented, "unknown service")

	holder, err := hold(ctx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	expectedTSO := uint64(physical) << tsoPhysicalShiftBits
	fake.mu.Lock()
	if got := fake.legacy[holder.ServiceID()]; got != expectedTSO {
		t.Fatalf("service safepoint at %d, expected %d", got, expectedTSO)
	}
	fake.mu.Unlock()

	if err := holder.Release(ctx); err != nil {
		t.Fatal(err)
	}

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if _, ok := fake.legacy[holder.ServiceID()]; ok {
		t.Fatal("service safepoint was not removed on release")
	}
}

func TestHold_RejectsSubSecondTTL(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	// The legacy API takes whole seconds, where a truncated 0 means "delete";
	// a sub-second TTL must be refused up front rather than letting every
	// heartbeat silently remove the hold. Hold rejects it before dialing PD.
	_, err := Hold(ctx, zaptest.NewLogger(t), SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         500 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected Hold to refuse a sub-second TTL")
	}
}

func TestHolder_RejectsAdvancedSafepoint(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	physical := time.Now().UnixMilli()
	fake := newFakePD(physical, 0)
	fake.barrierErr = status.Error(codes.Unimplemented, "unknown service")
	// the cluster GC safepoint is already past any timestamp we can request
	fake.minSafepoint = uint64(physical+1) << tsoPhysicalShiftBits

	_, err := hold(ctx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         time.Minute,
	})
	if err == nil {
		t.Fatal("expected hold to fail when the cluster safepoint is ahead")
	}
}

func TestHolder_RejectsBarrierError(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fake := newFakePD(time.Now().UnixMilli(), 0)
	fake.barrierErr = errs.New("txn safe point exceeds barrier ts")

	_, err := hold(ctx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         time.Minute,
	})
	if err == nil {
		t.Fatal("expected hold to fail on a non-Unimplemented barrier error")
	}
}

// TestHolder_AcquisitionDeadlineDoesNotOutliveAcquisition pins the separation
// between the bounded acquisition and the unbounded hold. Hold gives the
// initial connect and registration a deadline, so that an unreachable PD fails
// the run instead of retrying forever. Were that deadline to reach the
// heartbeat, the safepoint would silently stop being refreshed a minute into a
// scan that is meant to run for hours, and GC would collect the snapshot out
// from under it.
func TestHolder_AcquisitionDeadlineDoesNotOutliveAcquisition(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fake := newFakePD(time.Now().UnixMilli(), 0)

	// stands in for Hold's bounded acquisition context
	acquireCtx, cancelAcquire := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancelAcquire()

	holder, err := hold(acquireCtx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         300 * time.Millisecond, // refreshes every ttl/3
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Release(ctx) }()

	scanCtx := holder.Context(ctx)

	// let the acquisition deadline pass, as it does the moment Hold returns
	<-acquireCtx.Done()
	settled := fake.barrierSetCount()

	// the heartbeat has to keep refreshing well past it
	time.Sleep(500 * time.Millisecond)
	if refreshed := fake.barrierSetCount(); refreshed <= settled {
		t.Fatalf("no refresh after the acquisition deadline passed (%d calls, still %d): "+
			"the deadline followed the holder into the heartbeat", settled, refreshed)
	}

	// ...and the hold must not be reported lost
	select {
	case <-scanCtx.Done():
		t.Fatalf("scan context cancelled after the acquisition deadline: %v", context.Cause(scanCtx))
	default:
	}
}

func TestHolder_LostHoldCancelsContext(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	fake := newFakePD(time.Now().UnixMilli(), 0)

	holder, err := hold(ctx, zaptest.NewLogger(t), fake, SafepointConfig{
		PDEndpoints: "fake:2379",
		ServiceID:   "test",
		TTL:         600 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = holder.Release(ctx) }()

	scanCtx := holder.Context(ctx)

	// heartbeats start failing; the scan context must be cancelled before
	// the TTL can expire on the PD side
	fake.mu.Lock()
	fake.barrierErr = errs.New("pd unreachable")
	setsBeforeFailure := fake.barrierSets
	fake.mu.Unlock()

	select {
	case <-scanCtx.Done():
	case <-time.After(15 * time.Second):
		t.Fatal("scan context was not cancelled after heartbeats failed")
	}

	cause := context.Cause(scanCtx)
	if cause == nil || !Error.Has(cause) {
		t.Fatalf("expected hold-lost cause, got %v", cause)
	}

	// The abort must come no later than the second failed heartbeat
	// (~2/3 of the TTL after the last success). Waiting for a third failure
	// would mark the hold lost only at ~TTL, when PD may already have
	// released the safepoint under a still-running scan.
	if failed := fake.barrierSetCount() - setsBeforeFailure; failed > 2 {
		t.Fatalf("hold marked lost only after %d failed heartbeats; "+
			"the safepoint TTL may expire before the scan aborts", failed)
	}
}
