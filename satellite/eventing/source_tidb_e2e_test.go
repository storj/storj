// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package eventing_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	"storj.io/storj/private/server"
	"storj.io/storj/satellite/buckets"
	"storj.io/storj/satellite/eventing"
	"storj.io/storj/satellite/eventing/eventingconfig"
	"storj.io/storj/satellite/satellitedb"
	"storj.io/storj/satellite/satellitedb/satellitedbtest"
	"storj.io/storj/shared/dbutil/dbtest"
	"storj.io/storj/shared/mud"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/satellitetest"
	"storj.io/storj/shared/mudplanet/uplinktest"
	"storj.io/storj/shared/s3event"
	"storj.io/uplink"
	"storj.io/uplink/private/access"
	"storj.io/uplink/private/object"
)

// capturingPublisher collects published payloads for assertion.
type capturingPublisher struct {
	t      *testing.T
	mu     sync.Mutex
	events []eventing.Event
}

func (p *capturingPublisher) Publish(_ context.Context, data []byte, _ eventing.PublishMetadata) eventing.PendingResult {
	var event eventing.Event
	require.NoError(p.t, json.Unmarshal(data, &event))
	p.mu.Lock()
	p.events = append(p.events, event)
	p.mu.Unlock()
	return eventing.ImmediateResult(time.Now())
}

func (p *capturingPublisher) TopicName() string { return "@log" }
func (p *capturingPublisher) Close() error      { return nil }

func (p *capturingPublisher) reset() {
	p.mu.Lock()
	p.events = nil
	p.mu.Unlock()
}

func (p *capturingPublisher) waitForCount(t *testing.T, n int) []eventing.Event {
	t.Helper()
	require.Eventually(t, func() bool {
		p.mu.Lock()
		defer p.mu.Unlock()
		return len(p.events) >= n
	}, 5*time.Second, 20*time.Millisecond, "timed out waiting for %d events", n)
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]eventing.Event, len(p.events))
	copy(out, p.events)
	return out
}

func (p *capturingPublisher) assertNoEvents(t *testing.T) {
	t.Helper()
	time.Sleep(50 * time.Millisecond)
	p.mu.Lock()
	defer p.mu.Unlock()
	require.Empty(t, p.events, "expected no events but got %d", len(p.events))
}

var _ eventing.Publisher = &capturingPublisher{}

// tidbEventingConfig builds a mudplanet.Config that combines:
// - a satellite backed by TiDB (metabase) + Postgres (master)
// - n storage nodes registered in the satellite overlay
// - the eventing service with a capturing publisher
func tidbEventingConfig(t *testing.T, metabaseConnStr, masterConnStr string, n int, publisher *capturingPublisher) mudplanet.Config {
	t.Helper()

	tidbDB := satellitedbtest.SatelliteDatabases{
		Name:       "TiDB",
		MasterDB:   satellitedbtest.Database{Name: "Postgres", URL: masterConnStr},
		MetabaseDB: satellitedbtest.Database{Name: "TiDB", URL: metabaseConnStr},
	}

	// Build the base config with storage nodes (uses Postgres by default).
	cfg := satellitetest.WithStorageNodes(t, n)

	// Augment the satellite component with eventing config and the eventing service selector.
	// WithStorageNodes always places the satellite component first.
	sat := &cfg.Components[0]
	sat.Selector = mud.Or(sat.Selector, mud.SelectIfExists[*eventing.Service]())
	sat.PreInit = append(sat.PreInit,
		func(cfg *eventing.Config) {
			cfg.TiDBPollInterval = 10 * time.Millisecond
			cfg.TestNewPublisherFn = func() (eventing.Publisher, error) {
				return publisher, nil
			}
		},
		func(cfg *eventingconfig.Config) {
			cfg.Cache.TTL = 10 * time.Millisecond
		},
	)

	// Replace the RunWrapper so it supplies TiDB directly instead of iterating
	// all configured databases (which would skip TiDB).
	cfg.RunWrapper = func(t *testing.T, fn func(t *testing.T, module func(*mud.Ball))) {
		fn(t, func(ball *mud.Ball) {
			mud.Supply(ball, tidbDB)
		})
	}

	return cfg
}

func TestTiDBEventingEndToEnd(t *testing.T) {
	connStr := dbtest.PickTiDB(t)
	metabaseConnStr, masterConnStr, ok := strings.Cut(connStr, "!!master=")
	require.True(t, ok, "TiDB connection string must contain !!master=")

	publisher := &capturingPublisher{t: t}

	mudplanet.Run(t, tidbEventingConfig(t, metabaseConnStr, masterConnStr, 4, publisher),
		func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
			srv := mudplanet.FindFirst[*server.Server](t, run, "satellite", 0)
			id := mudplanet.FindFirst[*identity.FullIdentity](t, run, "satellite", 0)
			bucketsDB := mudplanet.FindFirst[buckets.DB](t, run, "satellite", 0)

			// The test project ID is fixed by satellitedb.GetTestApiKey.
			projectID := uuid.UUID([16]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})

			uplinkCfg := uplink.Config{}
			access.DisableObjectKeyEncryption(&uplinkCfg)

			acc, err := satellitedb.GetTestApiKey(ctx, uplinkCfg, id.ID, srv.Addr().String())
			require.NoError(t, err)

			ul, err := uplinktest.NewUplink(acc, uplinkCfg)
			require.NoError(t, err)

			proj, err := ul.OpenProject(ctx)
			require.NoError(t, err)
			defer func() { _ = proj.Close() }()

			// enableEventing sets up a bucket with notification config pointing to our publisher.
			enableEventing := func(t *testing.T, bucketName string) {
				t.Helper()
				err = bucketsDB.UpdateBucketNotificationConfig(ctx, []byte(bucketName), projectID, buckets.NotificationConfig{
					TopicName: "@log",
					Events:    []string{s3event.ObjectCreatedAll.S3Name(), s3event.ObjectRemovedAll.S3Name()},
				})
				require.NoError(t, err)
			}

			// Create buckets once and reuse across sub-tests.
			// bucket-a: eventing enabled (plain)
			// bucket-b: eventing enabled (plain, used as move destination)
			// bucket-silent: no notification config
			_, err = proj.EnsureBucket(ctx, "bucket-a")
			require.NoError(t, err)
			_, err = proj.EnsureBucket(ctx, "bucket-b")
			require.NoError(t, err)
			_, err = proj.EnsureBucket(ctx, "bucket-silent")
			require.NoError(t, err)
			enableEventing(t, "bucket-a")
			enableEventing(t, "bucket-b")

			t.Run("inline object upload and delete", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-a", "small-object", []byte("hi"))
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectCreatedPut.Name(), events[0].Records[0].EventName)
				require.Equal(t, "bucket-a", events[0].Records[0].S3.Bucket.Name)
				require.Equal(t, "small-object", events[0].Records[0].S3.Object.Key)

				err = ul.Delete(ctx, "bucket-a", "small-object")
				require.NoError(t, err)

				events = publisher.waitForCount(t, 2)
				require.Equal(t, s3event.ObjectRemovedDelete.Name(), events[1].Records[0].EventName)
			})

			t.Run("remote segment upload", func(t *testing.T) {
				publisher.reset()

				data := testrand.Bytes(5 * memory.KiB)
				err = ul.Upload(ctx, "bucket-a", "large-object", data)
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectCreatedPut.Name(), events[0].Records[0].EventName)
				require.Equal(t, "bucket-a", events[0].Records[0].S3.Bucket.Name)
				require.Equal(t, "large-object", events[0].Records[0].S3.Object.Key)
			})

			t.Run("no event when bucket has no notification config", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-silent", "silent-object", []byte("hi"))
				require.NoError(t, err)

				publisher.assertNoEvents(t)
			})

			t.Run("copy object", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-a", "copy-src", []byte("hi"))
				require.NoError(t, err)
				_ = publisher.waitForCount(t, 1) // consume the Put event

				publisher.reset()
				err = ul.Copy(ctx, "bucket-a", "copy-src", "bucket-a", "copy-dst")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectCreatedCopy.Name(), events[0].Records[0].EventName)
				require.Equal(t, "copy-dst", events[0].Records[0].S3.Object.Key)
			})

			t.Run("move object both buckets have eventing", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-a", "move-obj", []byte("hi"))
				require.NoError(t, err)
				_ = publisher.waitForCount(t, 1)

				publisher.reset()
				err = ul.Move(ctx, "bucket-a", "move-obj", "bucket-b", "move-obj")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 2)
				byName := map[string]eventing.EventRecord{}
				for _, e := range events {
					byName[e.Records[0].EventName] = e.Records[0]
				}
				require.Contains(t, byName, s3event.ObjectRemovedDelete.Name())
				require.Contains(t, byName, s3event.ObjectCreatedCopy.Name())
				require.Equal(t, "bucket-a", byName[s3event.ObjectRemovedDelete.Name()].S3.Bucket.Name)
				require.Equal(t, "bucket-b", byName[s3event.ObjectCreatedCopy.Name()].S3.Bucket.Name)
				require.Equal(t, "move-obj", byName[s3event.ObjectRemovedDelete.Name()].S3.Object.Key)
				require.Equal(t, "move-obj", byName[s3event.ObjectCreatedCopy.Name()].S3.Object.Key)
			})

			t.Run("move object only source has eventing", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-a", "move-src-only", []byte("hi"))
				require.NoError(t, err)
				_ = publisher.waitForCount(t, 1)

				publisher.reset()
				err = ul.Move(ctx, "bucket-a", "move-src-only", "bucket-silent", "move-src-only")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				// Wait one extra poll cycle to ensure no second event (ObjectCreated:Copy) arrives.
				time.Sleep(20 * time.Millisecond)
				require.Len(t, publisher.waitForCount(t, 1), 1)
				require.Equal(t, s3event.ObjectRemovedDelete.Name(), events[0].Records[0].EventName)
				require.Equal(t, "bucket-a", events[0].Records[0].S3.Bucket.Name)
			})

			t.Run("move object only destination has eventing", func(t *testing.T) {
				publisher.reset()

				err = ul.Upload(ctx, "bucket-silent", "move-dst-only", []byte("hi"))
				require.NoError(t, err)
				publisher.assertNoEvents(t)

				err = ul.Move(ctx, "bucket-silent", "move-dst-only", "bucket-b", "move-dst-only")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				// Wait one extra poll cycle to ensure no second event (ObjectRemoved:Delete) arrives.
				time.Sleep(20 * time.Millisecond)
				require.Len(t, publisher.waitForCount(t, 1), 1)
				require.Len(t, events, 1)
				require.Equal(t, s3event.ObjectCreatedCopy.Name(), events[0].Records[0].EventName)
				require.Equal(t, "bucket-b", events[0].Records[0].S3.Bucket.Name)
			})

			t.Run("versioned delete creates delete marker", func(t *testing.T) {
				publisher.reset()
				_, err = proj.EnsureBucket(ctx, "bucket-versioned")
				require.NoError(t, err)
				enableEventing(t, "bucket-versioned")

				err = bucketsDB.EnableBucketVersioning(ctx, []byte("bucket-versioned"), projectID)
				require.NoError(t, err)

				err = ul.Upload(ctx, "bucket-versioned", "obj", []byte("hi"))
				require.NoError(t, err)
				_ = publisher.waitForCount(t, 1)

				publisher.reset()
				err = ul.Delete(ctx, "bucket-versioned", "obj")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectRemovedDeleteMarkerCreated.Name(), events[0].Records[0].EventName)
			})

			t.Run("object lock delete exact version", func(t *testing.T) {
				publisher.reset()

				// Object lock requires versioning; create the bucket directly via DB.
				_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
					Name:       "bucket-lock-exact",
					ProjectID:  projectID,
					Versioning: buckets.VersioningEnabled,
					ObjectLock: buckets.ObjectLockSettings{Enabled: true},
				})
				require.NoError(t, err)
				enableEventing(t, "bucket-lock-exact")

				upload, err := object.UploadObject(ctx, proj, "bucket-lock-exact", "obj", nil)
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
				obj := upload.Info()
				_ = publisher.waitForCount(t, 1)

				publisher.reset()
				_, err = object.DeleteObject(ctx, proj, "bucket-lock-exact", "obj", obj.Version, nil)
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectRemovedDelete.Name(), events[0].Records[0].EventName)
			})

			t.Run("object lock delete last committed", func(t *testing.T) {
				publisher.reset()

				_, err = bucketsDB.CreateBucket(ctx, buckets.Bucket{
					Name:       "bucket-lock-last",
					ProjectID:  projectID,
					Versioning: buckets.VersioningEnabled,
					ObjectLock: buckets.ObjectLockSettings{Enabled: true},
				})
				require.NoError(t, err)
				enableEventing(t, "bucket-lock-last")

				upload, err := object.UploadObject(ctx, proj, "bucket-lock-last", "obj", nil)
				require.NoError(t, err)
				require.NoError(t, upload.Commit())
				_ = publisher.waitForCount(t, 1)

				publisher.reset()
				// Delete without specifying version on a versioned bucket creates a delete marker.
				_, err = proj.DeleteObject(ctx, "bucket-lock-last", "obj")
				require.NoError(t, err)

				events := publisher.waitForCount(t, 1)
				require.Equal(t, s3event.ObjectRemovedDeleteMarkerCreated.Name(), events[0].Records[0].EventName)
			})
		})
}
