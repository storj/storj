// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/pprof"
	"time"

	"github.com/dsnet/try"
	"github.com/spacemonkeygo/monkit/v3"
	"github.com/spacemonkeygo/monkit/v3/collect"
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
	"storj.io/storj/storagenode/collector"
	"storj.io/storj/storagenode/contact"
	"storj.io/storj/storagenode/monitor"
	"storj.io/storj/storagenode/orders"
	"storj.io/storj/storagenode/pieces"
	"storj.io/storj/storagenode/piecestore"
	"storj.io/storj/storagenode/piecestore/usedserials"
	"storj.io/storj/storagenode/retain"
	"storj.io/storj/storagenode/storagenodedb"
	"storj.io/storj/storagenode/trust"
)

var (
	pieceSize       = flag.Int("piece-size", 62068, "62068 bytes for a piece in a 1.8 MB file. must be less than 100MiB")
	piecesToUpload  = flag.Int("pieces-to-upload", 10000, "")
	workers         = flag.Int("workers", 5, "")
	ttl             = flag.Duration("ttl", time.Hour, "")
	forceSync       = flag.Bool("force-sync", false, "")
	disablePrealloc = flag.Bool("disable-prealloc", false, "")

	mon  = monkit.Package()
	data []byte
)

func createEndpoint(ctx context.Context, satIdent, snIdent *identity.FullIdentity) (*piecestore.Endpoint, *collector.Service) {
	log := zap.L()

	var cfg storagenode.Config
	setConfigStructDefaults(&cfg)
	if *disablePrealloc {
		cfg.Pieces.WritePreallocSize = -1
	}
	cfg.Filestore.ForceSync = *forceSync

	resolver := trust.IdentityResolverFunc(func(ctx context.Context, url storj.NodeURL) (*identity.PeerIdentity, error) {
		if url.ID == satIdent.ID {
			return satIdent.PeerIdentity(), nil
		}
		return nil, fmt.Errorf("unknown peer id")
	})

	try.E(cfg.Storage2.Trust.Sources.Set(fmt.Sprintf("%s@localhost:0", satIdent.ID)))

	snDB := try.E1(storagenodedb.OpenNew(ctx, log, cfg.DatabaseConfig()))
	try.E(snDB.MigrateToLatest(ctx))

	trustPool := try.E1(trust.NewPool(log, resolver, cfg.Storage2.Trust, snDB.Satellites()))
	try.E(trustPool.Refresh(ctx))

	blobsCache := pieces.NewBlobsUsageCache(log, snDB.Pieces())
	filewalker := pieces.NewFileWalker(log, blobsCache, snDB.V0PieceInfo(),
		snDB.GCFilewalkerProgress())

	piecesStore := pieces.NewStore(log, filewalker, nil, blobsCache, snDB.V0PieceInfo(), snDB.PieceExpirationDB(), snDB.PieceSpaceUsedDB(), cfg.Pieces)

	tlsOptions := try.E1(tlsopts.NewOptions(snIdent, cfg.Server.Config, nil))

	dialer := rpc.NewDefaultDialer(tlsOptions)

	self := contact.NodeInfo{ID: snIdent.ID}

	contactService := contact.NewService(log, dialer, self, trustPool, contact.NewQUICStats(false), &pb.SignedNodeTagSets{})

	monitorService := monitor.NewService(log, piecesStore, contactService, snDB.Bandwidth(), 1<<40, time.Hour, func(context.Context) {}, cfg.Storage2.Monitor)

	retainService := retain.NewService(log, piecesStore, cfg.Retain)

	trashChore := pieces.NewTrashChore(log, 24*time.Hour, 7*24*time.Hour, trustPool, piecesStore)

	pieceDeleter := pieces.NewDeleter(log, piecesStore, cfg.Storage2.DeleteWorkers, cfg.Storage2.DeleteQueueSize)

	ordersStore := try.E1(orders.NewFileStore(log, cfg.Storage2.Orders.Path, cfg.Storage2.OrderLimitGracePeriod))

	usedSerials := usedserials.NewTable(cfg.Storage2.MaxUsedSerialsSize)

	return try.E1(piecestore.NewEndpoint(log, snIdent, trustPool, monitorService, retainService, new(contact.PingStats), piecesStore, trashChore, pieceDeleter, ordersStore, snDB.Bandwidth(), usedSerials, cfg.Storage2)),
		collector.NewService(log, piecesStore, usedSerials, collector.Config{Interval: 1000 * time.Hour})
}

func createUpload(ctx context.Context, satIdent, snIdent *identity.FullIdentity) *stream {
	defer mon.Task()(&ctx)(nil)
	piecePubKey, piecePrivKey := try.E2(storj.NewPieceKey())
	rootPieceId := storj.NewPieceID()

	var expiration time.Time
	if *ttl > 0 {
		expiration = time.Now().Add(*ttl)
	}

	limit := try.E1(signing.SignOrderLimit(ctx, signing.SignerFromFullIdentity(satIdent), &pb.OrderLimit{
		SerialNumber:    try.E1(satorders.CreateSerial(time.Now().Add(time.Hour))),
		SatelliteId:     satIdent.ID,
		UplinkPublicKey: piecePubKey,
		StorageNodeId:   snIdent.ID,

		PieceId: rootPieceId.Deriver().Derive(snIdent.ID, 0),
		Limit:   100 * (1 << 20),
		Action:  pb.PieceAction_PUT,

		PieceExpiration: expiration,
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(time.Hour),

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

func uploadPiece(ctx context.Context, endpoint *piecestore.Endpoint, upload *stream) []*collect.FinishedSpan {
	defer mon.Task()(&ctx)(nil)

	return collect.CollectSpans(ctx, func(ctx context.Context) {
		defer mon.TaskNamed("start-upload-piece")(&ctx)(nil)
		upload.ctx = ctx
		try.E(endpoint.Upload(upload))
	})
}

func runCollector(ctx context.Context, collector *collector.Service) []*collect.FinishedSpan {
	defer mon.Task()(&ctx)(nil)

	return collect.CollectSpans(ctx, func(ctx context.Context) {
		defer mon.TaskNamed("start-collect")(&ctx)(nil)
		try.E(collector.Collect(ctx, time.Now().Add(7*24*time.Hour+*ttl)))
	})
}

func main() {
	flag.Parse()
	ctx := context.Background()
	data = try.E1(io.ReadAll(io.LimitReader(rand.Reader, int64(*pieceSize))))

	satIdent := try.E1(identity.NewFullIdentity(ctx, identity.NewCAOptions{Difficulty: 0, Concurrency: 1}))

	snIdent := try.E1(identity.NewFullIdentity(ctx, identity.NewCAOptions{Difficulty: 0, Concurrency: 1}))

	endpoint, collector := createEndpoint(ctx, satIdent, snIdent)

	allUploads := make([]*stream, 0, *piecesToUpload)
	for i := 0; i < *piecesToUpload; i++ {
		allUploads = append(allUploads, createUpload(ctx, satIdent, snIdent))
	}

	// allSpans := make([][]*collect.FinishedSpan, 0, *piecesToUpload)
	start := time.Now()

	// if *cpuprofile != "" {
	f, err := os.Create("cpu.pprof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()
	// }

	queue := make(chan *stream, 100)
	spans := make(chan []*collect.FinishedSpan, 100)
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
		<-spans
	}

	uploadDuration := time.Since(start)
	fmt.Printf("uploaded %d %s pieces in %s (%0.02f MiB/s, %0.02f pieces/s)\n",
		*piecesToUpload, memory.Size(*pieceSize).Base10String(), uploadDuration,
		float64((*pieceSize)*(*piecesToUpload))/(1024*1024*uploadDuration.Seconds()),
		float64(*piecesToUpload)/uploadDuration.Seconds())

	// allSpans = append(allSpans, runCollector(ctx, collector)
	runCollector(ctx, collector)

	collectDuration := time.Since(start) - uploadDuration
	fmt.Printf("collected %d pieces in %s (%0.02f MiB/s)\n", *piecesToUpload, collectDuration, float64((*pieceSize)*(*piecesToUpload))/(1024*1024*collectDuration.Seconds()))

	// statsfh := try.E1(os.Create("stats.txt"))
	// try.E(present.StatsText(monkit.Default, statsfh))
	// try.E(statsfh.Close())

	// funcsfh := try.E1(os.Create("funcs.dot"))
	// try.E(present.FuncsDot(monkit.Default, funcsfh))
	// try.E(funcsfh.Close())

	// tracefh := try.E1(os.Create("traces.json"))
	// try.E1(tracefh.WriteString("[\n"))
	// for i, spans := range allSpans {
	// 	if i > 0 {
	// 		try.E1(tracefh.WriteString(",\n"))
	// 	}
	// 	try.E(present.SpansToJSON(tracefh, spans))
	// }
	// try.E1(tracefh.WriteString("]"))
	// try.E(tracefh.Close())
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

func setConfigStructDefaults(v interface{}) {
	fs := pflag.NewFlagSet("defaults", pflag.ContinueOnError)
	cfgstruct.Bind(fs, v, cfgstruct.ConfDir("."), cfgstruct.IdentityDir("."), cfgstruct.UseReleaseDefaults())
	try.E(fs.Parse(nil))
}
