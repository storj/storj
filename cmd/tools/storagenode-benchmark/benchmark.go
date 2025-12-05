// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"time"

	"github.com/dsnet/try"
	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"
	"github.com/zeebo/errs"
	"golang.org/x/exp/slices"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/rpc/quic"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/uplink/private/piecestore"
)

var (
	runBenchmarkCmd = &cobra.Command{
		Use:   "benchmark",
		Short: "upload/download benchmark against storage node",
		RunE:  runBenchmark,
	}

	benchmarkConfig BenchmarkConfig
)

// BenchmarkConfig is the configuration for the benchmark command.
type BenchmarkConfig struct {
	PiecesToUpload int
	Workers        int
	TTL            time.Duration

	Noise  bool
	Pooled bool

	HashAlgorithm int
	NodeURL       string
	PieceSize     string
	IdentityDir   string
	NoiseInfoKey  string

	CPUProfile string
}

// BindFlags adds flags to the flagset.
func (config *BenchmarkConfig) BindFlags(flag *flag.FlagSet) {
	flag.IntVar(&config.PiecesToUpload, "pieces-to-upload", 10000, "")
	flag.IntVar(&config.Workers, "workers", 5, "")
	flag.DurationVar(&config.TTL, "ttl", time.Hour, "")
	flag.StringVar(&config.PieceSize, "piece-size", "64KiB", "")

	flag.BoolVar(&config.Noise, "noise", true, "")
	flag.StringVar(&config.NoiseInfoKey, "noise-info-key", "", "hex encoded noise info key")
	flag.BoolVar(&config.Pooled, "pooled", true, "")

	flag.IntVar(&config.HashAlgorithm, "hash-algorithm", 1, "")

	flag.StringVar(&config.NodeURL, "node-url", "", "")
	flag.StringVar(&config.IdentityDir, "identity-dir", "", "")

	flag.StringVar(&config.CPUProfile, "cpuprofile", "", "")
}

func init() {
	rootCmd.AddCommand(runBenchmarkCmd)

	benchmarkConfig.BindFlags(runBenchmarkCmd.Flags())
}

func runBenchmark(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	orderLimitCreator, err := newKeySignerFromDir(benchmarkConfig.IdentityDir, pb.PieceAction_PUT, benchmarkConfig.TTL)
	if err != nil {
		return errs.Wrap(err)
	}

	pieceSize := try.E1(memory.ParseString(benchmarkConfig.PieceSize))
	data := try.E1(io.ReadAll(io.LimitReader(rand.Reader, pieceSize)))

	helper := dialerHelper{
		Noise:       benchmarkConfig.Noise,
		Pooled:      benchmarkConfig.Pooled,
		IdentityDir: benchmarkConfig.IdentityDir,
	}

	dialer, err := helper.CreateRPCDialer()
	if err != nil {
		return errs.Wrap(err)
	}

	nodeURL, err := storj.ParseNodeURL(benchmarkConfig.NodeURL)
	if err != nil {
		return errs.Wrap(err)
	}

	if benchmarkConfig.NoiseInfoKey != "" {
		nodeURL.NoiseInfo.Proto = storj.NoiseProto(pb.NoiseProtocol_NOISE_IK_25519_CHACHAPOLY_BLAKE2B)
		key, err := hex.DecodeString(benchmarkConfig.NoiseInfoKey)
		if err != nil {
			return errs.Wrap(err)
		}
		nodeURL.NoiseInfo.PublicKey = string(key)
	}

	pieceIDQueue := make(chan storj.PieceID, benchmarkConfig.Workers)

	allPieceIds := make([]storj.PieceID, 0, benchmarkConfig.PiecesToUpload)
	for i := 0; i < benchmarkConfig.PiecesToUpload; i++ {
		allPieceIds = append(allPieceIds, storj.NewPieceID())
	}

	var uwg sync.WaitGroup
	duration := profile(benchmarkConfig.CPUProfile, "upload", func() {
		for i := 0; i < benchmarkConfig.Workers; i++ {
			uwg.Add(1)
			data := slices.Clone(data)

			go func() {
				defer uwg.Done()
				for pieceID := range pieceIDQueue {
					if pieceID.IsZero() {
						return
					}
					if err := connectAndUpload(ctx, dialer, orderLimitCreator, nodeURL, pieceID, benchmarkConfig.HashAlgorithm, data); err != nil {
						fmt.Println(err)
					}
				}
			}()
		}

		for _, pieceID := range allPieceIds {
			pieceIDQueue <- pieceID
		}

		for i := 0; i < benchmarkConfig.Workers; i++ {
			pieceIDQueue <- storj.PieceID{}
		}
		uwg.Wait()
	})

	fmt.Printf("BenchmarkUpload/%s-%d\t%d\t%0.02f ns/op\t%0.02f MiB/s\t%0.02f pieces/s\n",
		strings.ReplaceAll(memory.Size(pieceSize).Base10String(), " ", ""),
		benchmarkConfig.Workers,
		benchmarkConfig.PiecesToUpload,
		float64(duration)/float64(benchmarkConfig.PiecesToUpload),
		float64(pieceSize)*float64(benchmarkConfig.PiecesToUpload)/(1024*1024*duration.Seconds()),
		float64(benchmarkConfig.PiecesToUpload)/duration.Seconds())

	var dwg sync.WaitGroup
	pieceIDQueue = make(chan storj.PieceID, benchmarkConfig.Workers)
	duration = profile(benchmarkConfig.CPUProfile, "download", func() {
		for i := 0; i < benchmarkConfig.Workers; i++ {
			dwg.Add(1)
			go func() {
				defer dwg.Done()
				for pieceID := range pieceIDQueue {
					if pieceID.IsZero() {
						return
					}
					if err := connectAndDownload(ctx, dialer, orderLimitCreator, nodeURL, pieceID, pieceSize); err != nil {
						fmt.Println(err)
					}
				}
			}()
		}

		for _, pieceID := range allPieceIds {
			pieceIDQueue <- pieceID
		}
		for i := 0; i < benchmarkConfig.Workers; i++ {
			pieceIDQueue <- storj.PieceID{}
		}
		dwg.Wait()
	})

	close(pieceIDQueue)

	fmt.Printf("BenchmarkDownload/%s-%d\t%d\t%0.02f ns/op\t%0.02f MiB/s\t%0.02f pieces/s\n",
		strings.ReplaceAll(memory.Size(pieceSize).Base10String(), " ", ""),
		benchmarkConfig.Workers,
		benchmarkConfig.PiecesToUpload,
		float64(duration)/float64(benchmarkConfig.PiecesToUpload),
		float64(pieceSize)*float64(benchmarkConfig.PiecesToUpload)/(1024*1024*duration.Seconds()),
		float64(benchmarkConfig.PiecesToUpload)/duration.Seconds())

	return nil
}

func connectAndUpload(ctx context.Context, d rpc.Dialer, orderLimitCreator *keySigner, nodeURL storj.NodeURL, pieceID storj.PieceID, hashAlgo int, data []byte) (err error) {
	client, err := piecestore.Dial(ctx, d, nodeURL, piecestore.DefaultConfig)
	if err != nil {
		return errs.Wrap(err)
	}
	client.UploadHashAlgo = pb.PieceHashAlgorithm(hashAlgo)
	defer func() {
		err = errs.Combine(err, client.Close())
	}()

	limit, privateKey, _, err := orderLimitCreator.createOrderLimit(ctx, pieceID, int64(len(data)), nodeURL.ID)
	_, err = client.UploadReader(ctx, limit, privateKey, bytes.NewReader(data))
	return errs.Wrap(err)
}

func connectAndDownload(ctx context.Context, d rpc.Dialer, orderLimitCreator *keySigner, nodeURL storj.NodeURL, pieceID storj.PieceID, pieceSize int64) (err error) {
	client, err := piecestore.Dial(ctx, d, nodeURL, piecestore.DefaultConfig)
	if err != nil {
		return errs.Wrap(err)
	}

	defer func() {
		err = errs.Combine(err, client.Close())
	}()

	limit, privateKey, _, err := orderLimitCreator.createGetOrderLimit(ctx, pieceID, pieceSize, nodeURL.ID)
	download, err := client.Download(ctx, limit, privateKey, 0, pieceSize)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		err = errs.Combine(err, download.Close())
	}()

	n, err := io.Copy(io.Discard, download)
	if err != nil {
		return errs.Wrap(err)
	}
	if n != pieceSize {
		return errs.New("downloaded %d bytes, expected %d", n, pieceSize)
	}
	return nil
}

type keySigner struct {
	nodeID storj.NodeID
	signer signing.Signer
	action pb.PieceAction
	ttl    time.Duration
}

func newKeySignerFromDir(keysDir string, action pb.PieceAction, ttl time.Duration) (*keySigner, error) {
	satelliteIdentityCfg := identity.Config{
		CertPath: filepath.Join(keysDir, "identity.cert"),
		KeyPath:  filepath.Join(keysDir, "identity.key"),
	}
	id, err := satelliteIdentityCfg.Load()
	if err != nil {
		return nil, err
	}
	return &keySigner{
		nodeID: id.ID,
		signer: signing.SignerFromFullIdentity(id),
		action: action,
		ttl:    ttl,
	}, nil
}

func (k *keySigner) createOrderLimit(ctx context.Context, pieceID storj.PieceID, size int64, sn storj.NodeID) (limit *pb.OrderLimit, pk storj.PiecePrivateKey, serial storj.SerialNumber, err error) {
	pub, pk, err := storj.NewPieceKey()
	if err != nil {
		return
	}
	_, err = rand.Read(serial[:])
	if err != nil {
		return
	}

	limit = &pb.OrderLimit{
		PieceId:         pieceID,
		SerialNumber:    serial,
		SatelliteId:     k.nodeID,
		StorageNodeId:   sn,
		Action:          pb.PieceAction_PUT,
		Limit:           size,
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(24 * time.Hour),
		UplinkPublicKey: pub,
	}
	if benchmarkConfig.TTL > 0 {
		limit.PieceExpiration = time.Now().Add(benchmarkConfig.TTL)
	}
	limit, err = signing.SignOrderLimit(ctx, k.signer, limit)
	if err != nil {
		return
	}
	return
}

func (k *keySigner) createGetOrderLimit(ctx context.Context, pieceID storj.PieceID, size int64, sn storj.NodeID) (limit *pb.OrderLimit, pk storj.PiecePrivateKey, serial storj.SerialNumber, err error) {
	pub, pk, err := storj.NewPieceKey()
	if err != nil {
		return
	}
	_, err = rand.Read(serial[:])
	if err != nil {
		return
	}

	limit = &pb.OrderLimit{
		PieceId:         pieceID,
		SerialNumber:    serial,
		SatelliteId:     k.nodeID,
		StorageNodeId:   sn,
		Action:          pb.PieceAction_GET,
		Limit:           size,
		OrderCreation:   time.Now(),
		UplinkPublicKey: pub,
	}
	limit, err = signing.SignOrderLimit(ctx, k.signer, limit)
	if err != nil {
		return
	}
	return
}

type dialerHelper struct {
	Quic        bool
	Pooled      bool
	Noise       bool
	IdentityDir string `default:"."`
}

func (d *dialerHelper) CreateRPCDialer() (rpc.Dialer, error) {
	var err error
	var ident *identity.FullIdentity
	if _, err := os.Stat(filepath.Join(d.IdentityDir, "identity.cert")); err == nil || d.IdentityDir != "." {
		satelliteIdentityCfg := identity.Config{
			CertPath: filepath.Join(d.IdentityDir, "identity.cert"),
			KeyPath:  filepath.Join(d.IdentityDir, "identity.key"),
		}
		ident, err = satelliteIdentityCfg.Load()
		if err != nil {
			return rpc.Dialer{}, err
		}
	} else {
		return rpc.Dialer{}, err
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return rpc.Dialer{}, err
	}
	var dialer rpc.Dialer
	if d.Pooled {
		dialer = rpc.NewDefaultPooledDialer(tlsOptions)
	} else {
		dialer = rpc.NewDefaultDialer(tlsOptions)
	}

	dialer.DialTimeout = 1 * time.Second

	if d.Quic {
		dialer.Connector = quic.NewDefaultConnector(nil)
	} else {
		dialer.Connector = rpc.NewHybridConnector()
	}
	return dialer, nil
}

func profile(cpuprofile string, suffix string, fn func()) time.Duration {
	if cpuprofile != "" {
		cpufile := try.E1(os.Create(cpuprofile + "." + suffix))
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
