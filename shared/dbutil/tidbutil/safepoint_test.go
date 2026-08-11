// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package tidbutil

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pd "github.com/tikv/pd/client"
	"github.com/tikv/pd/client/clients/gc"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"storj.io/common/testcontext"
)

// fakeClusterIDs hands every fakePD a distinct cluster, the way separate PDs
// are distinct; a test that wants two labels on one cluster shares the ID.
var fakeClusterIDs atomic.Uint64

type fakePD struct {
	mu sync.Mutex

	physical  int64
	logical   int64
	clusterID uint64

	barrierErr   error // returned by SetGCBarrier when set
	barriers     map[string]uint64
	barrierSets  int // counts SetGCBarrier calls, so refreshes are observable
	legacy       map[string]uint64
	minSafepoint uint64
	closed       bool
	closes       int // counts Close calls, so releasing one cluster twice is visible
}

// barrierSetCount reports how many times SetGCBarrier has been called.
func (f *fakePD) barrierSetCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.barrierSets
}

func newFakePD(physical, logical int64) *fakePD {
	return &fakePD{
		physical:  physical,
		logical:   logical,
		clusterID: fakeClusterIDs.Add(1),
		barriers:  map[string]uint64{},
		legacy:    map[string]uint64{},
	}
}

func (f *fakePD) GetTS(ctx context.Context) (int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.physical, f.logical, nil
}

func (f *fakePD) GetClusterID(ctx context.Context) uint64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.clusterID
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
	f.closes++
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

// TestHold_RejectsPartialTLS covers the configurations that would otherwise
// dial PD in plaintext. The PD client turns TLS on only when it has both a
// certificate and a key, so a CA-only config is silently insecure: against a
// TLS-enabled PD it fails as a connect timeout an hour into debugging, and
// against a plaintext PD it succeeds while looking like it verified something.
func TestHold_RejectsPartialTLS(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	for _, tt := range []struct {
		name   string
		config SafepointConfig
	}{
		{"CA only", SafepointConfig{CAPath: "ca.crt"}},
		{"cert without key", SafepointConfig{CAPath: "ca.crt", CertPath: "client.crt"}},
		{"key without cert", SafepointConfig{CAPath: "ca.crt", KeyPath: "client.key"}},
		{"cert and key without CA", SafepointConfig{CertPath: "client.crt", KeyPath: "client.key"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.PDEndpoints = "fake:2379"
			config.ServiceID = "test"
			config.TTL = time.Minute

			// rejected before dialing, so no PD is needed
			if _, err := Hold(ctx, zaptest.NewLogger(t), config); err == nil {
				t.Fatal("expected Hold to refuse a partial TLS configuration")
			}
		})
	}
}

// TestSecurityOptions covers spreading the TLS paths over the clusters:
// isolated clusters may each have their own certificate authority, while the
// usual deployment has one for all of them.
func TestSecurityOptions(t *testing.T) {
	labels := []string{"west", "east"}

	for _, tt := range []struct {
		name   string
		config SafepointConfig
		want   map[string]pd.SecurityOption
	}{
		{
			name:   "no TLS",
			config: SafepointConfig{},
			want: map[string]pd.SecurityOption{
				"west": {},
				"east": {},
			},
		},
		{
			name:   "shared by every cluster",
			config: SafepointConfig{CAPath: "ca.crt", CertPath: "client.crt", KeyPath: "client.key"},
			want: map[string]pd.SecurityOption{
				"west": {CAPath: "ca.crt", CertPath: "client.crt", KeyPath: "client.key"},
				"east": {CAPath: "ca.crt", CertPath: "client.crt", KeyPath: "client.key"},
			},
		},
		{
			name: "one certificate authority per cluster",
			config: SafepointConfig{
				CAPath:   "west=/certs/west/ca.crt; east=/certs/east/ca.crt",
				CertPath: "west=/certs/west/client.crt;east=/certs/east/client.crt",
				KeyPath:  "west=/certs/west/client.key;east=/certs/east/client.key",
			},
			want: map[string]pd.SecurityOption{
				"west": {CAPath: "/certs/west/ca.crt", CertPath: "/certs/west/client.crt", KeyPath: "/certs/west/client.key"},
				"east": {CAPath: "/certs/east/ca.crt", CertPath: "/certs/east/client.crt", KeyPath: "/certs/east/client.key"},
			},
		},
		{
			// the settings resolve independently, so one deployment-wide CA
			// combines with a client certificate per cluster
			name: "shared CA with per-cluster client certificates",
			config: SafepointConfig{
				CAPath:   "ca.crt",
				CertPath: "west=/certs/west/client.crt;east=/certs/east/client.crt",
				KeyPath:  "west=/certs/west/client.key;east=/certs/east/client.key",
			},
			want: map[string]pd.SecurityOption{
				"west": {CAPath: "ca.crt", CertPath: "/certs/west/client.crt", KeyPath: "/certs/west/client.key"},
				"east": {CAPath: "ca.crt", CertPath: "/certs/east/client.crt", KeyPath: "/certs/east/client.key"},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.config.securityOptions(labels)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestSecurityOptions_GroupedLabels covers the metabase with more backends than
// clusters: the backends sharing a cluster name their path once, the way they
// name their PD endpoints once, rather than repeating it under a label each.
func TestSecurityOptions_GroupedLabels(t *testing.T) {
	// PD endpoints of the same shape: "west=...;east+south=..."
	config := SafepointConfig{
		CAPath:   "west=/certs/west/ca.crt;east+south=/certs/east/ca.crt",
		CertPath: "/certs/client.crt",
		KeyPath:  "/certs/client.key",
	}
	want := map[string]pd.SecurityOption{
		"west":  {CAPath: "/certs/west/ca.crt", CertPath: "/certs/client.crt", KeyPath: "/certs/client.key"},
		"east":  {CAPath: "/certs/east/ca.crt", CertPath: "/certs/client.crt", KeyPath: "/certs/client.key"},
		"south": {CAPath: "/certs/east/ca.crt", CertPath: "/certs/client.crt", KeyPath: "/certs/client.key"},
	}

	got, err := config.securityOptions([]string{"west", "east", "south"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	// a path carrying a "+" but no label is still one plain path for everyone
	plain, err := SafepointConfig{
		CAPath:   "/certs/a+b/ca.crt",
		CertPath: "/certs/a+b/client.crt",
		KeyPath:  "/certs/a+b/client.key",
	}.securityOptions([]string{"west", "east"})
	if err != nil {
		t.Fatal(err)
	}
	if plain["west"].CAPath != "/certs/a+b/ca.crt" {
		t.Fatalf("CA path %q, want the path unsplit", plain["west"].CAPath)
	}
}

// TestSecurityOptions_Invalid covers the labeled configurations that would
// leave a cluster dialed with something other than what was intended for it.
func TestSecurityOptions_Invalid(t *testing.T) {
	full := func(config SafepointConfig) SafepointConfig {
		if config.CAPath == "" {
			config.CAPath = "ca.crt"
		}
		if config.CertPath == "" {
			config.CertPath = "client.crt"
		}
		if config.KeyPath == "" {
			config.KeyPath = "client.key"
		}
		return config
	}

	for _, tt := range []struct {
		name   string
		config SafepointConfig
	}{
		// a cluster missing from a labeled list would be dialed in plaintext
		{"backend missing from the CA list", full(SafepointConfig{CAPath: "west=/certs/west/ca.crt"})},
		{"backend missing from the cert list", full(SafepointConfig{CertPath: "west=/certs/west/client.crt"})},
		{"backend missing from the key list", full(SafepointConfig{KeyPath: "west=/certs/west/client.key"})},
		// a typo in a label would otherwise pass unnoticed as the case above
		{"unknown backend", full(SafepointConfig{CAPath: "west=/certs/west/ca.crt;west2=/certs/east/ca.crt"})},
		{"duplicate backend", full(SafepointConfig{CAPath: "west=/certs/west/ca.crt;west=/certs/east/ca.crt"})},
		// a grouped entry is still a list of labels and every one of them counts
		{"unknown backend in a group", full(SafepointConfig{CAPath: "west+west2=/certs/west/ca.crt;east=/certs/east/ca.crt"})},
		{"duplicate backend across groups", full(SafepointConfig{CAPath: "west+east=/certs/west/ca.crt;east=/certs/east/ca.crt"})},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.config.securityOptions([]string{"west", "east"}); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestSplitEndpoints(t *testing.T) {
	for _, tt := range []struct {
		endpoints string
		sep       string
		want      []string
	}{
		{" 10.0.0.1:2389, 10.0.0.2:2389 ,,10.0.0.3:2389", ",",
			[]string{"10.0.0.1:2389", "10.0.0.2:2389", "10.0.0.3:2389"}},
		// one entry per cluster, each its own comma-separated PD list
		{"10.0.0.1:2389,10.0.0.2:2389; 10.1.0.1:2389 ;", ";",
			[]string{"10.0.0.1:2389,10.0.0.2:2389", "10.1.0.1:2389"}},
	} {
		got := splitEndpoints(tt.endpoints, tt.sep)
		if len(got) != len(tt.want) {
			t.Fatalf("got %q, want %q", got, tt.want)
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		}
	}
}

// TestHolds_MultipleClusters covers the setup where the metabase is spread over
// isolated TiDB clusters with a PD each: every cluster is pinned, and the single
// timestamp the scan reads at is the newest of them, so that it is at or above
// every barrier rather than below the barrier of the cluster that is ahead.
func TestHolds_MultipleClusters(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	now := time.Now().UnixMilli()

	// the east cluster's clock runs 300ms behind the west one
	clusters := map[string]*fakePD{"west": newFakePD(now, 0), "east": newFakePD(now-300, 0)}

	holds := Holds{}
	for label, fake := range clusters {
		holder, err := hold(ctx, log, fake, SafepointConfig{ServiceID: "test", TTL: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		holds[label] = holder
	}

	if got, want := holds.ReadTime(), time.UnixMilli(now).UTC(); !got.Equal(want) {
		t.Fatalf("read time %v, want the newest cluster timestamp %v", got, want)
	}
	readTSO := uint64(holds.ReadTime().UnixMilli()) << tsoPhysicalShiftBits
	for label, fake := range clusters {
		fake.mu.Lock()
		barrier, ok := fake.barriers[holds[label].ServiceID()]
		fake.mu.Unlock()
		if !ok {
			t.Fatalf("backend %q was not pinned", label)
		}
		if readTSO < barrier {
			t.Fatalf("read tso %d is below the barrier %d of backend %q; "+
				"the read is unprotected once the barrier binds", readTSO, barrier, label)
		}
	}

	// the lagging cluster has not reached the read timestamp yet, so aligning
	// must wait for its clock rather than let the scan read the future
	aligned := make(chan error, 1)
	go func() { aligned <- holds.alignReadTime(ctx) }()
	select {
	case err := <-aligned:
		t.Fatalf("aligned while the east cluster was still 300ms behind: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	east := clusters["east"]
	east.mu.Lock()
	east.physical = now + 1
	east.mu.Unlock()

	select {
	case err := <-aligned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("alignReadTime did not return after the lagging clock caught up")
	}

	if err := holds.Release(ctx); err != nil {
		t.Fatal(err)
	}
	for label, fake := range clusters {
		fake.mu.Lock()
		if len(fake.barriers) != 0 || !fake.closed {
			t.Fatalf("hold on %q not released: barriers %v, closed %v", label, fake.barriers, fake.closed)
		}
		fake.mu.Unlock()
	}
}

// TestHolds_RejectsSameClusterTwice covers the copy-paste that adding a second
// cluster invites: a second label pointing at the first cluster's PD. Nothing
// else catches it -- the labels differ and they match the metabase backends --
// while it takes two barriers on one cluster and leaves the other unpinned, so
// the scan reads that one live.
func TestHolds_RejectsSameClusterTwice(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	now := time.Now().UnixMilli()

	west, east := newFakePD(now, 0), newFakePD(now, 0)
	east.clusterID = west.clusterID // both labels ended up on the same PD

	holds := Holds{}
	for label, fake := range map[string]*fakePD{"west": west, "east": east} {
		holder, err := hold(ctx, log, fake, SafepointConfig{ServiceID: "test", TTL: time.Minute})
		if err != nil {
			t.Fatal(err)
		}
		holds[label] = holder
	}
	defer func() { _ = holds.Release(ctx) }()

	if err := holds.verifyDistinctClusters(ctx); err == nil {
		t.Fatal("two backends pointing at the same cluster were accepted; one of them is unpinned")
	}

	// ...while genuinely separate clusters are fine
	east.clusterID = west.clusterID + 1
	if err := holds.verifyDistinctClusters(ctx); err != nil {
		t.Fatal(err)
	}
}

// TestHolds_SharedCluster covers backends that really do live on one cluster --
// separate schemas of the same TiDB, which is how a metabase gets more backends
// than there are clusters. One entry names them both, so there is one hold, and
// everything acting on the cluster has to act on it once however many backends
// point at it.
func TestHolds_SharedCluster(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)
	now := time.Now().UnixMilli()

	// "west=...;east+south=..." -- east and south are two schemas of one cluster
	west, shared := newFakePD(now, 0), newFakePD(now, 0)
	westHold, err := hold(ctx, log, west, SafepointConfig{ServiceID: "test", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	sharedHold, err := hold(ctx, log, shared, SafepointConfig{ServiceID: "test", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	holds := Holds{"west": westHold, "east": sharedHold, "south": sharedHold}

	// the shared cluster is one cluster, not two backends colliding on one
	if err := holds.verifyDistinctClusters(ctx); err != nil {
		t.Fatal(err)
	}

	// both clocks read now, so there is nothing to wait for; the point is that
	// three backends over two clusters still compares two clocks
	if err := holds.alignReadTime(ctx); err != nil {
		t.Fatal(err)
	}

	if got, want := holds.ServiceIDs(), []string{
		"east+south=" + sharedHold.ServiceID(), "west=" + westHold.ServiceID(),
	}; !slices.Equal(got, want) {
		t.Fatalf("service ids %v, want the shared cluster named once as %v", got, want)
	}

	if err := holds.Release(ctx); err != nil {
		t.Fatal(err)
	}
	for name, fake := range map[string]*fakePD{"west": west, "east+south": shared} {
		fake.mu.Lock()
		barriers, closes := len(fake.barriers), fake.closes
		fake.mu.Unlock()
		if barriers != 0 {
			t.Fatalf("hold on %q not released: %d barriers left", name, barriers)
		}
		// releasing the shared cluster once per backend would delete a barrier
		// that is already gone and close a client that is already closed
		if closes != 1 {
			t.Fatalf("cluster %q closed %d times, want exactly once", name, closes)
		}
	}
}

// TestHolds_SharedClusterAlone covers the single cluster carrying every backend:
// there is one clock, so there is nothing for the read timestamp to catch up to
// even though the labels outnumber the clusters.
func TestHolds_SharedClusterAlone(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	fake := newFakePD(time.Now().UnixMilli(), 0)
	holder, err := hold(ctx, log, fake, SafepointConfig{ServiceID: "test", TTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	holds := Holds{"east": holder, "south": holder}
	defer func() { _ = holds.Release(ctx) }()

	// with a lagging clock this would be the one cluster waiting for itself
	fake.mu.Lock()
	fake.physical -= 300
	fake.mu.Unlock()

	aligned := make(chan error, 1)
	go func() { aligned <- holds.alignReadTime(ctx) }()
	select {
	case err := <-aligned:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("alignReadTime waited for the only cluster to catch up with itself")
	}
}

// TestSharedSecurity covers backends on one cluster given different TLS
// material: the cluster is dialed once, so one of the two settings would be
// silently ignored.
func TestSharedSecurity(t *testing.T) {
	security := map[string]pd.SecurityOption{
		"east":  {CAPath: "/ca.crt", CertPath: "/east.crt", KeyPath: "/east.key"},
		"south": {CAPath: "/ca.crt", CertPath: "/south.crt", KeyPath: "/south.key"},
	}
	if _, err := sharedSecurity("east+south", []string{"east", "south"}, security); err == nil {
		t.Fatal("backends sharing a cluster were accepted with different certificates")
	}

	security["south"] = security["east"]
	got, err := sharedSecurity("east+south", []string{"east", "south"}, security)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, security["east"]) {
		t.Fatalf("security option %v, want %v", got, security["east"])
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
