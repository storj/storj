// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"io"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/dsnet/try"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/collect"
	"github.com/spacemonkeygo/monkit/v3/present"
	"github.com/spf13/pflag"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/drpc"
	satorders "storj.io/storj/satellite/orders"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/bandwidth"
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/hashstore"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/piecestore/usedserials"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/trust"
)

const orderExpiration = 24 * time.Hour

var (
	pieceSize          = flag.Int("piece-size", 62068, "62068 bytes for a piece in a 1.8 MB file. must be less than 100MiB")
	piecesToUpload     = flag.Int("pieces-to-upload", 10000, "")
	workers            = flag.Int("workers", 5, "")
	ttl                = flag.Duration("ttl", time.Hour, "")
	forceSync          = flag.Bool("force-sync", false, "")
	disablePrealloc    = flag.Bool("disable-prealloc", false, "")
	workDir            = flag.String("work-dir", "", "")
	dbsLocation        = flag.String("dbs-location", "", "")
	flatFileTTLStore   = flag.Bool("flat-ttl-store", true, "use flat-files ttl store")
	flatFileTTLHandles = flag.Int("flat-ttl-max-handles", 1000, "max file handles to flat-file ttl store")
	dedicatedDisk      = flag.Bool("dedicated-disk", false, "assume the test setup is a dedicated disk node")

	backend = flag.String("backend", "", "empty|hash|hashstore")

	cpuprofile = flag.String("cpuprofile", "", "write a cpu profile")
	memprofile = flag.String("memprofile", "", "write a memory profile")
	notrace    = flag.Bool("notrace", false, "disable tracing")

	skipUploads   = flag.Bool("skip-uploads", false, "skip uploads")
	skipDownloads = flag.Bool("skip-downloads", false, "skip downloads")
	skipCollect   = flag.Bool("skip-collect", false, "skip collect")

	mon  = monkit.Package()
	data []byte
)

func createIdentity(ctx context.Context, path string) *identity.FullIdentity {
	var cfg identity.Config
	cfg.CertPath = filepath.Join(path, "identity.cert")
	cfg.KeyPath = filepath.Join(path, "identity.key")
	ident, err := cfg.Load()
	if err != nil {
		ident = try.E1(identity.NewFullIdentity(ctx, identity.NewCAOptions{Difficulty: 0, Concurrency: 1}))
		try.E(cfg.Save(ident))
	}
	return ident
}

func createEndpoint(ctx context.Context, satIdent, snIdent *identity.FullIdentity) (*piecestore.Endpoint, *collector.Service) {
	log := zap.L()

	var cfg storagenode.Config
	setConfigStructDefaults(&cfg)
	if *disablePrealloc {
		cfg.Pieces.WritePreallocSize = -1
	}
	cfg.Filestore.ForceSync = *forceSync
	cfg.Storage2.OrderLimitGracePeriod = orderExpiration

	resolver := trust.IdentityResolverFunc(func(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error) {
		if url.ID == satIdent.ID {
			return satIdent.PeerIdentity(), nil
		}
		return nil, errors.New("unknown peer id")
	})

	try.E(cfg.Storage2.Trust.Sources.Set(fmt.Sprintf("%s@localhost:0", satIdent.ID)))

	if *dbsLocation != "" {
		cfg.Storage2.DatabaseDir = *dbsLocation
		try.E(os.MkdirAll(*dbsLocation, 0755))
	}

	snDB, err := storagenodedb.OpenNew(ctx, log, cfg.DatabaseConfig())
	if err != nil {
		snDB = try.E1(storagenodedb.OpenExisting(ctx, log, cfg.DatabaseConfig()))
	}
	try.E(snDB.MigrateToLatest(ctx))

	trustPool := try.E1(trust.NewPool(log, resolver, cfg.Storage2.Trust, snDB.Satellites()))
	try.E(trustPool.Refresh(ctx))

	blobStore := snDB.Pieces()
	if !*dedicatedDisk {
		blobStore = pieces.NewBlobsUsageCache(log, snDB.Pieces())
	}
	filewalker := pieces.NewFileWalker(log, blobStore, snDB.V0PieceInfo(),
		snDB.GCFilewalkerProgress(), snDB.UsedSpacePerPrefix())

	var expirationStore pieces.PieceExpirationDB
	if *flatFileTTLStore {
		cfg.Pieces.EnableFlatExpirationStore = true
		expirationStore = try.E1(pieces.NewPieceExpirationStore(log.Named("piece-expiration"), pieces.PieceExpirationConfig{
			DataDir:               filepath.Join(cfg.Storage2.DatabaseDir, "pieceexpiration"),
			ConcurrentFileHandles: *flatFileTTLHandles,
		}))
	} else {
		expirationStore = snDB.PieceExpirationDB()
	}
	piecesStore := pieces.NewStore(log, filewalker, nil, blobStore, snDB.V0PieceInfo(), expirationStore, cfg.Pieces)

	tlsOptions := try.E1(tlsopts.NewOptions(snIdent, cfg.Server.Config, nil))

	dialer := rpc.NewDefaultDialer(tlsOptions)

	self := contact.NodeInfo{ID: snIdent.ID}

	contactService := contact.NewService(log, dialer, self, trustPool, contact.NewQUICStats(false), &pb.SignedNodeTagSets{})

	retainService := retain.NewService(log, piecesStore, cfg.Retain)

	trashChore := pieces.NewTrashChore(log, 24*time.Hour, 7*24*time.Hour, trustPool, piecesStore)

	ordersStore := try.E1(orders.NewFileStore(log, cfg.Storage2.Orders.Path, cfg.Storage2.OrderLimitGracePeriod))

	usedSerials := usedserials.NewTable(cfg.Storage2.MaxUsedSerialsSize)

	bandwidthdbCache := bandwidth.NewCache(snDB.Bandwidth())

	bfm := try.E1(retain.NewBloomFilterManager("bfm", cfg.Retain.MaxTimeSkew))

	rtm := retain.NewRestoreTimeManager("rtm")
	// TODO: use injected configuration
	hsb := try.E1(piecestore.NewHashStoreBackend(ctx, hashstore.CreateDefaultConfig(hashstore.TableKind_HashTbl, false), "hashstore", "", bfm, rtm, log))
	mon.Chain(hsb)

	var spaceReport monitor.SpaceReport
	if *dedicatedDisk {
		spaceReport = monitor.NewDedicatedDisk(log, cfg.Storage.Path, cfg.Storage2.Monitor.MinimumDiskSpace.Int64(), 100_000_000)
	} else {
		spaceReport = monitor.NewSharedDisk(log, piecesStore, hsb, cfg.Storage2.Monitor.MinimumDiskSpace.Int64(), 1<<40)
	}

	monitorService := monitor.NewService(log, piecesStore, contactService, spaceReport, cfg.Storage2.Monitor, cfg.Contact.CheckInTimeout)

	opb := piecestore.NewOldPieceBackend(piecesStore, trashChore, monitorService)

	var pieceBackend piecestore.PieceBackend
	switch *backend {
	case "hashstore", "hash":
		pieceBackend = hsb
	default:
		pieceBackend = opb
	}

	endpoint := try.E1(piecestore.NewEndpoint(log, snIdent, trustPool, monitorService, []piecestore.QueueRetain{retainService, bfm}, new(contact.PingStats), pieceBackend, ordersStore, bandwidthdbCache, usedSerials, nil, cfg.Storage2))
	collectorService := collector.NewService(log, piecesStore, usedSerials, collector.Config{Interval: 1000 * time.Hour})

	return endpoint, collectorService
}

func createPieceID(n int) storj.PieceID {
	// Convert the n to a byte slice
	bytes := make([]byte, 8) // int64 is 8 bytes
	binary.BigEndian.PutUint64(bytes, uint64(n))

	// Create the piece id from the sha256 hash of the byte slice
	return storj.PieceID(sha256.Sum256(bytes))
}

func createUpload(ctx context.Context, satIdent, snIdent *identity.FullIdentity, pieceID storj.PieceID) *stream {
	defer mon.Task()(&ctx)(nil)
	piecePubKey, piecePrivKey := try.E2(storj.NewPieceKey())

	var expiration time.Time
	if *ttl > 0 {
		expiration = time.Now().Add(*ttl)
	}

	limit := try.E1(signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satIdent), &pb.OrderLimit{
		SerialNumber:    try.E1(satorders.CreateSerial(time.Now().Add(orderExpiration))),
		SatelliteId:     satIdent.ID,
		UplinkPublicKey: piecePubKey,
		StorageNodeId:   snIdent.ID,

		PieceId: pieceID,
		Limit:   100 * (1 << 20),
		Action:  pb.PieceAction_PUT,

		PieceExpiration: expiration,
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(orderExpiration),

		EncryptedMetadataKeyId: try.E1(io.ReadAll(io.LimitReader(rand.Reader, 16))),
		EncryptedMetadata:      try.E1(io.ReadAll(io.LimitReader(rand.Reader, 32))),
	}))

	algo := pb.PieceHashAlgorithm_BLAKE3
	hash := pb.NewHashFromAlgorithm(algo)
	try.E1(hash.Write(data))

	return &stream{
		messages: []*pb.PieceUploadRequest{
			{
				Limit:         limit,
				HashAlgorithm: algo,
				Order: try.E1(signing.SignUplinkOrder(ctx, piecePrivKey, &pb.Order{
					SerialNumber: limit.SerialNumber,
					Amount:       int64(len(data)),
				})),
				Chunk: &pb.PieceUploadRequest_Chunk{
					Offset: 0,
					Data:   data,
				},
				Done: try.E1(signing.SignUplinkPieceHash(ctx, piecePrivKey, &pb.PieceHash{
					PieceId:       limit.PieceId,
					PieceSize:     int64(len(data)),
					Hash:          hash.Sum(nil),
					Timestamp:     limit.OrderCreation,
					HashAlgorithm: algo,
				})),
			},
		}}
}

func createDownload(ctx context.Context, satIdent, snIdent *identity.FullIdentity, pieceID storj.PieceID) *downloadStream {
	defer mon.Task()(&ctx)(nil)
	piecePubKey, piecePrivKey := try.E2(storj.NewPieceKey())

	limit := try.E1(signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satIdent), &pb.OrderLimit{
		SerialNumber:    try.E1(satorders.CreateSerial(time.Now().Add(orderExpiration))),
		SatelliteId:     satIdent.ID,
		UplinkPublicKey: piecePubKey,
		StorageNodeId:   snIdent.ID,

		PieceId: pieceID,
		Limit:   100 * (1 << 20),
		Action:  pb.PieceAction_GET,

		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(orderExpiration),

		EncryptedMetadataKeyId: try.E1(io.ReadAll(io.LimitReader(rand.Reader, 16))),
		EncryptedMetadata:      try.E1(io.ReadAll(io.LimitReader(rand.Reader, 32))),
	}))

	return &downloadStream{
		messages: []*pb.PieceDownloadRequest{
			{
				Limit: limit,
				Order: try.E1(signing.SignUplinkOrder(ctx, piecePrivKey, &pb.Order{
					SerialNumber: limit.SerialNumber,
					Amount:       int64(len(data)),
				})),
				Chunk: &pb.PieceDownloadRequest_Chunk{
					Offset:    0,
					ChunkSize: int64(len(data)),
				},
			},
		},
		ch: make(chan interface{})}
}

func uploadPiece(ctx context.Context, endpoint *piecestore.Endpoint, upload *stream) []*collect.FinishedSpan {
	defer mon.Task()(&ctx)(nil)

	if *notrace {
		try.E(endpoint.Upload(upload))
		return nil
	}

	return collect.CollectSpans(ctx, func(ctx context.Context) {
		defer mon.TaskNamed("start-upload-piece")(&ctx)(nil)
		upload.ctx = ctx
		try.E(endpoint.Upload(upload))
	})
}

func downloadPiece(ctx context.Context, endpoint *piecestore.Endpoint, download *downloadStream) []*collect.FinishedSpan {
	defer mon.Task()(&ctx)(nil)

	if *notrace {
		try.E(endpoint.Download(download))
		return nil
	}

	return collect.CollectSpans(ctx, func(ctx context.Context) {
		defer mon.TaskNamed("start-download-piece")(&ctx)(nil)
		download.ctx = ctx
		try.E(endpoint.Download(download))
	})
}

func runCollector(ctx context.Context, collector *collector.Service) []*collect.FinishedSpan {
	defer mon.Task()(&ctx)(nil)

	if *notrace {
		try.E(collector.Collect(ctx, time.Now().Add(7*24*time.Hour+*ttl)))
		return nil
	}

	return collect.CollectSpans(ctx, func(ctx context.Context) {
		defer mon.TaskNamed("start-collect")(&ctx)(nil)
		try.E(collector.Collect(ctx, time.Now().Add(7*24*time.Hour+*ttl)))
	})
}

func main() {
	ctx := context.Background()

	flag.Parse()

	if *workDir != "" {
		try.E(os.MkdirAll(*workDir, 0755))
		try.E(os.Chdir(*workDir))
	}

	data = try.E1(io.ReadAll(io.LimitReader(rand.Reader, int64(*pieceSize))))

	satIdent := createIdentity(ctx, "identity/satellite")
	snIdent := createIdentity(ctx, "identity/storagenode")

	endpoint, collector := createEndpoint(ctx, satIdent, snIdent)

	allPieceIDs := make([]storj.PieceID, 0, *piecesToUpload)
	for i := 0; i < *piecesToUpload; i++ {
		pieceID := createPieceID(i)
		allPieceIDs = append(allPieceIDs, pieceID)
	}

	allSpans := make([][]*collect.FinishedSpan, 0, len(allPieceIDs))

	if !*skipUploads {
		allUploads := make([]*stream, 0, len(allPieceIDs))
		for _, pieceID := range allPieceIDs {
			upload := createUpload(ctx, satIdent, snIdent, pieceID)
			allUploads = append(allUploads, upload)
		}

		queue := make(chan *stream, 100)
		spans := make(chan []*collect.FinishedSpan, 100)

		duration := profile("upload", func() {
			for i := 0; i < *workers; i++ {
				go func() {
					for upload := range queue {
						spans <- uploadPiece(ctx, endpoint, upload)
					}
				}()
			}
			go func() {
				for _, upload := range allUploads {
					queue <- upload
				}
				close(queue)
			}()

			for i := 0; i < *piecesToUpload; i++ {
				allSpans = append(allSpans, <-spans)
			}
		})

		fmt.Printf("BenchmarkUpload/%s-%d\t%d\t%0.02f ns/op\t%0.02f MiB/s\t%0.02f pieces/s\n",
			strings.ReplaceAll(memory.Size(*pieceSize).Base10String(), " ", ""),
			*workers,
			*piecesToUpload,
			float64(duration)/float64(*piecesToUpload),
			float64((*pieceSize)*(*piecesToUpload))/(1024*1024*duration.Seconds()),
			float64(*piecesToUpload)/duration.Seconds())
	}

	if !*skipDownloads {
		allDownloads := make([]*downloadStream, 0, len(allPieceIDs))
		for _, pieceID := range allPieceIDs {
			download := createDownload(ctx, satIdent, snIdent, pieceID)
			allDownloads = append(allDownloads, download)
		}

		// shuffle the downloads to ensure random reads
		mathrand.Shuffle(len(allDownloads), func(i, j int) {
			allDownloads[i], allDownloads[j] = allDownloads[j], allDownloads[i]
		})

		queue := make(chan *downloadStream, 100)
		spans := make(chan []*collect.FinishedSpan, 100)

		duration := profile("download", func() {
			for i := 0; i < *workers; i++ {
				go func() {
					for download := range queue {
						spans <- downloadPiece(ctx, endpoint, download)
					}
				}()
			}
			go func() {
				for _, download := range allDownloads {
					queue <- download
				}
				close(queue)
			}()

			for i := 0; i < *piecesToUpload; i++ {
				allSpans = append(allSpans, <-spans)
			}
		})

		fmt.Printf("BenchmarkDownload/%s-%d\t%d\t%0.02f ns/op\t%0.02f MiB/s\t%0.02f pieces/s\n",
			strings.ReplaceAll(memory.Size(*pieceSize).Base10String(), " ", ""),
			*workers,
			*piecesToUpload,
			float64(duration)/float64(*piecesToUpload),
			float64((*pieceSize)*(*piecesToUpload))/(1024*1024*duration.Seconds()),
			float64(*piecesToUpload)/duration.Seconds())
	}

	if !*skipCollect {
		duration := profile("collect", func() {
			allSpans = append(allSpans, runCollector(ctx, collector))
		})

		fmt.Printf("BenchmarkCollect/%s\t%d\t%0.04f ns/op\t%0.02f MiB/s\t%0.02f pieces/s\n",
			strings.ReplaceAll(memory.Size(*pieceSize).Base10String(), " ", ""),
			*piecesToUpload,
			float64(duration)/float64(*piecesToUpload),
			float64((*pieceSize)*(*piecesToUpload))/(1024*1024*duration.Seconds()),
			float64(*piecesToUpload)/duration.Seconds())
	}

	if !*notrace {
		statsfh := try.E1(os.Create("stats.txt"))
		try.E(present.StatsText(monkit.Default, statsfh))
		try.E(statsfh.Close())

		funcsfh := try.E1(os.Create("funcs.dot"))
		try.E(present.FuncsDot(monkit.Default, funcsfh))
		try.E(funcsfh.Close())

		tracefh := try.E1(os.Create("traces.json"))
		try.E1(tracefh.WriteString("[\n"))
		for i, spans := range allSpans {
			if i > 0 {
				try.E1(tracefh.WriteString(",\n"))
			}
			try.E(present.SpansToJSON(tracefh, spans))
		}
		try.E1(tracefh.WriteString("]"))
		try.E(tracefh.Close())
	}
}

type stream struct {
	ctx      context.Context
	messages []*pb.PieceUploadRequest
}

func (s *stream) SendAndClose(resp *pb.PieceUploadResponse) error {
	return nil
}

func (s *stream) Recv() (*pb.PieceUploadRequest, error) {
	if len(s.messages) == 0 {
		return nil, io.EOF
	}
	resp := s.messages[0]
	s.messages = s.messages[1:]
	return resp, nil
}

func (s *stream) Context() context.Context { return s.ctx }

func (s *stream) MsgSend(msg drpc.Message, enc drpc.Encoding) error { panic("unimplemented") }
func (s *stream) MsgRecv(msg drpc.Message, enc drpc.Encoding) error { panic("unimplemented") }
func (s *stream) CloseSend() error                                  { panic("unimplemented") }
func (s *stream) Close() error                                      { panic("unimplemented") }
func (s *stream) Cancel(err error) bool                             { panic("unimplemented") }

type downloadStream struct {
	ctx      context.Context
	messages []*pb.PieceDownloadRequest
	ch       chan interface{}
}

func (s *downloadStream) SendAndClose(resp *pb.PieceUploadResponse) error {
	return nil
}

func (s *downloadStream) Send(*pb.PieceDownloadResponse) error {
	s.ch <- struct{}{}
	return nil
}

func (s *downloadStream) Recv() (*pb.PieceDownloadRequest, error) {
	if len(s.messages) == 0 {
		// don't send EOF until the client has received the response
		<-s.ch
		close(s.ch)
		return nil, io.EOF
	}
	resp := s.messages[0]
	s.messages = s.messages[1:]
	return resp, nil
}

func (s *downloadStream) Context() context.Context { return s.ctx }

func (s *downloadStream) MsgSend(msg drpc.Message, enc drpc.Encoding) error { panic("unimplemented") }
func (s *downloadStream) MsgRecv(msg drpc.Message, enc drpc.Encoding) error { panic("unimplemented") }
func (s *downloadStream) CloseSend() error                                  { panic("unimplemented") }
func (s *downloadStream) Close() error                                      { panic("unimplemented") }
func (s *downloadStream) Cancel(err error) bool                             { panic("unimplemented") }

func setConfigStructDefaults(v interface{}) {
	fs := pflag.NewFlagSet("defaults", pflag.ContinueOnError)
	cfgstruct.Bind(fs, v, cfgstruct.ConfDir("."), cfgstruct.IdentityDir("."), cfgstruct.UseReleaseDefaults())
	try.E(fs.Parse(nil))
}

func profile(suffix string, fn func()) time.Duration {
	if *memprofile != "" {
		defer func() {
			memfile := try.E1(os.Create(uniqueProfilePath(*memprofile, suffix)))
			runtime.GC()
			try.E(pprof.WriteHeapProfile(memfile))
			try.E(memfile.Close())
		}()
	}

	if *cpuprofile != "" {
		cpufile := try.E1(os.Create(uniqueProfilePath(*cpuprofile, suffix)))
		defer func() { try.E(cpufile.Close()) }()
		try.E(pprof.StartCPUProfile(cpufile))

		defer func() {
			pprof.StopCPUProfile()
		}()
	}

	start := time.Now()
	fn()
	return time.Since(start)
}

var startTime = time.Now()

func uniqueProfilePath(template, suffix string) string {
	ext := filepath.Ext(template)
	prefix := strings.TrimSuffix(template, ext)

	return prefix + "." + startTime.Format("2006-01-02.15-04-05") + "." + suffix + ext
}
