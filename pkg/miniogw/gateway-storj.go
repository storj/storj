// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"path"
	"strconv"
	"time"

	"storj.io/storj/pkg/piecestore"

	"github.com/golang/protobuf/proto"
	"github.com/minio/cli"
	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
	"github.com/vivint/infectious"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/eestream"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/piecestore/rpc/client"
	"storj.io/storj/pkg/ranger"
	pb "storj.io/storj/protos/netstate"
	opb "storj.io/storj/protos/overlay"
)

var (
	pieceBlockSize = flag.Int("piece_block_size", 4*1024, "block size of pieces")
	key            = flag.String("key", "a key", "the secret key")
	rsk            = flag.Int("required", 5, "rs required")
	rsn            = flag.Int("total", 20, "rs total")
	rsm            = flag.Int("minimum", 0, "rs minimum safe")
	rso            = flag.Int("optimal", 0, "rs optimal safe")

	mon   = monkit.Package()
	Error = errs.Class("error")
)

var nodes = []string{
	"35.232.47.152:4242",
	"35.226.188.0:4242",
	"35.202.182.77:4242",
	"35.232.88.171:4242",
	"35.225.50.137:4242",
	"130.211.168.182:4242",
	"104.197.5.134:4242",
	"35.192.126.41:4242",
	"35.193.196.52:4242",
	"130.211.203.52:4242",
	"35.224.122.76:4242",
	"104.198.233.25:4242",
	"35.226.21.152:4242",
	"130.211.221.239:4242",
	"35.202.144.175:4242",
	"130.211.190.250:4242",
	"35.194.56.141:4242",
	"35.192.108.107:4242",
	"35.202.91.191:4242",
	"35.192.7.33:4242",
}

// nodeIds maps address to piece id (shards v2 name)
var nodeIds = map[string]string{}

// nodeAddrs maps piece id(shards) to address
var nodeAddrs = map[string]string{}

func init() {
	for i, address := range nodes {
		nodeAddrs[fmt.Sprint(i)] = address
		nodeIds[address] = fmt.Sprint(i)
	}

	minio.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})
}

// getBuckets returns the buckets list
func (s *storjObjects) getBuckets() (buckets []minio.BucketInfo, err error) {
	buckets = make([]minio.BucketInfo, len(s.storj.bucketlist))
	for i, bi := range s.storj.bucketlist {
		buckets[i] = minio.BucketInfo{
			Name:    bi.bucket.Name,
			Created: bi.bucket.Created,
		}
	}
	return buckets, nil
}

// uploadFile function handles to add the uploaded file to the bucket's file list structure
func (s *storjObjects) uploadFile(bucket, object string, filesize int64, metadata map[string]string) (result minio.ListObjectsInfo, err error) {
	var fl []minio.ObjectInfo
	for i, v := range s.storj.bucketlist {
		// bucket string comparision
		if v.bucket.Name == bucket {
			/* append the file to the filelist */
			s.storj.bucketlist[i].filelist.file.Objects = append(
				s.storj.bucketlist[i].filelist.file.Objects,
				minio.ObjectInfo{
					Bucket:      bucket,
					Name:        object,
					ModTime:     time.Now(),
					Size:        filesize,
					IsDir:       false,
					ContentType: "video/mp4",
				},
			)
			/* populate the filelist */
			f := make([]minio.ObjectInfo, len(s.storj.bucketlist[i].filelist.file.Objects))
			for j, fi := range s.storj.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      v.bucket.Name,
					Name:        fi.Name,
					ModTime:     fi.ModTime,
					Size:        fi.Size,
					IsDir:       fi.IsDir,
					ContentType: fi.ContentType,
				}
			}
			fl = f
			break
		}
	}
	result = minio.ListObjectsInfo{
		IsTruncated: false,
		Objects:     fl,
	}
	return result, nil
}

// getFiles returns the files list for a bucket
func (s *storjObjects) getFiles(bucket string) (result minio.ListObjectsInfo, err error) {
	var fl []minio.ObjectInfo
	for i, v := range s.storj.bucketlist {
		if v.bucket.Name == bucket {
			/* populate the filelist */
			f := make([]minio.ObjectInfo, len(s.storj.bucketlist[i].filelist.file.Objects))
			for j, fi := range s.storj.bucketlist[i].filelist.file.Objects {
				f[j] = minio.ObjectInfo{
					Bucket:      v.bucket.Name,
					Name:        fi.Name,
					ModTime:     fi.ModTime,
					Size:        fi.Size,
					IsDir:       fi.IsDir,
					ContentType: fi.ContentType,
				}
			}
			fl = f
			break
		}
	}
	result = minio.ListObjectsInfo{
		IsTruncated: false,
		Objects:     fl,
	}
	return result, nil
}

// deleteFile returns the files list for a bucket
func (s *storjObjects) deleteFile(bucket, object string) (err error) {
	for i, v := range s.storj.bucketlist {
		k := 0
		if v.bucket.Name == bucket {
			for j := 0; j < len(s.storj.bucketlist[i].filelist.file.Objects); j++ {
				fi := s.storj.bucketlist[i].filelist.file.Objects[j].Name
				fmt.Println("fi=", fi)
				if fi != object {
					s.storj.bucketlist[i].filelist.file.Objects[k].Name = fi
					k++
				}
			}
			s.storj.bucketlist[i].filelist.file.Objects = s.storj.bucketlist[i].filelist.file.Objects[:k]
		}
	}
	return nil
}

// removeBucket returns the files list for a bucket
func (s *storjObjects) removeBucket(bucket string) (err error) {
	k := 0
	for i, v := range s.storj.bucketlist {
		bi := s.storj.bucketlist[i].bucket.Name
		fmt.Println("bi=", bi)
		if v.bucket.Name != bucket {
			s.storj.bucketlist[k] = v
			k++
		}
	}
	s.storj.bucketlist = s.storj.bucketlist[:k]
	return nil
}

// addBucket returns the files list for a bucket
func (s *storjObjects) addBucket(bucket string) (err error) {
	s.storj.bucketlist = append(s.storj.bucketlist,
		S3Bucket{
			minio.BucketInfo{
				Name:    bucket,
				Created: time.Now(),
			},
			S3FileList{
				minio.ListObjectsInfo{},
			},
		},
	)
	return nil
}

func storjGatewayMain(ctx *cli.Context) {
	s := &Storj{}
	s.createSampleBucketList()
	minio.StartGateway(ctx, s)
}

//S3Bucket structure
type S3Bucket struct {
	bucket   minio.BucketInfo
	filelist S3FileList
}

//S3FileList structure
type S3FileList struct {
	file minio.ListObjectsInfo
}

// Storj is the implementation of a minio cmd.Gateway
type Storj struct {
	bucketlist []S3Bucket
}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	minio.ObjectLayer, error) {
	return &storjObjects{storj: s}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

//createSampleBucketList function initializes sample buckets and files in each bucket
func (s *Storj) createSampleBucketList() {
	s.bucketlist = make([]S3Bucket, 10)
	for i := range s.bucketlist {
		s.bucketlist[i].bucket.Name = "TestBucket" + strconv.Itoa(i+1)
		s.bucketlist[i].bucket.Created = time.Now()
		s.bucketlist[i].filelist.file.IsTruncated = false
		s.bucketlist[i].filelist.file.Objects = make([]minio.ObjectInfo, 0x0A)
		for j := range s.bucketlist[i].filelist.file.Objects {
			s.bucketlist[i].filelist.file.Objects[j].Bucket = s.bucketlist[i].bucket.Name
			s.bucketlist[i].filelist.file.Objects[j].Name = s.bucketlist[i].bucket.Name + "file" + strconv.Itoa(j+1)
			s.bucketlist[i].filelist.file.Objects[j].ModTime = time.Now()
			s.bucketlist[i].filelist.file.Objects[j].Size = 100
			s.bucketlist[i].filelist.file.Objects[j].ContentType = "application/octet-stream"
		}
	}
}

type storjObjects struct {
	minio.GatewayUnsupported
	TempDir string // Temporary storage location for file transfers.
	storj   *Storj
}

func (s *storjObjects) DeleteBucket(ctx context.Context, bucket string) error {
	return s.removeBucket(bucket)
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket,
	object string) error {
	return s.deleteFile(bucket, object)
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo minio.BucketInfo, err error) {
	panic("TODO")
}

//nodes []*proto.RemotePiece,
func netstateGet(ctx context.Context, path string) (rv *pb.Pointer, err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := grpc.Dial(*netstateAddress, grpc.WithInsecure())
	if err != nil {
		zap.L().Error("Failed to dial: ", zap.Error(err))
		return nil, err
	}

	client := pb.NewNetStateClient(conn)

	//zap.L().Debug(fmt.Sprintf("client dialed port %s", netstateAddress))

	// Example pointer paths to put
	//pr1 passes with api creds
	gr := pb.GetRequest{
		Path:   []byte(path), //key
		APIKey: []byte("abc123"),
	}

	// Example Puts
	// puts passes api creds
	getRes, err := client.Get(ctx, &gr)
	if err != nil || status.Code(err) == codes.Internal {
		zap.L().Error("failed to get", zap.Error(err))
		return nil, err
	}
	var p pb.Pointer
	err = proto.Unmarshal(getRes.Pointer, &p)
	if err != nil {
		zap.L().Error("failed to marshal", zap.Error(err))
		return nil, err
	}
	prettyprint(&p)
	return &p, nil
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {
	defer func() {
		if err != nil {
			fmt.Printf("%+v", Error.Wrap(err))
		}
	}()

	encKey := sha256.Sum256([]byte(*key))

	/* making request to network state to get the piece IDs */
	/* create a unique fileID */
	netStateKey := []byte(*key)
	encryptedPath, err := paths.Encrypt([]string{bucket, object}, netStateKey)

	pointer, err := netstateGet(ctx, path.Join(encryptedPath...))
	if err != nil {
		return err
	}

	if pointer.GetType() != pb.Pointer_REMOTE {
		return fmt.Errorf("TODO: only remote supported!")
	}
	remote := pointer.GetRemote()
	redundancy := remote.GetRedundancy()
	if redundancy.GetType() != pb.RedundancyScheme_RS {
		return fmt.Errorf("TODO: only rs supported!")
	}

	fc, err := infectious.NewFEC(int(redundancy.GetMinReq()), int(redundancy.GetTotal()))
	if err != nil {
		return err
	}
	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	var firstNonce [12]byte
	decrypter, err := eestream.NewAESGCMDecrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}

	addr := "bootstrap.storj.io:7070"
	c, err := overlay.NewClient(&addr, grpc.WithInsecure())
	if err != nil {
		return Error.Wrap(err)
	}

	errs := make(chan error, redundancy.GetTotal())
	conns := make([]*grpc.ClientConn, redundancy.GetTotal())
	for i := 0; i < len(remote.GetRemotePieces()); i++ {
		go func(i int) {
			resp, err := c.Lookup(ctx, &opb.LookupRequest{NodeID: remote.RemotePieces[i].NodeId})
			if err != nil {
				errs <- err
				return
			}

			conns[i], err = grpc.Dial(resp.NodeAddress.Address, grpc.WithInsecure())
			errs <- err
		}(i)
	}
	for range conns {
		err := <-errs
		if err != nil {
			// TODO don't fail just because of failure with only one node
			return err
		}
	}
	defer func() {
		for i := 0; i < len(remote.GetRemotePieces()); i++ {
			defer conns[i].Close()
		}
	}()
	rrs := map[int]ranger.Ranger{}
	type rangerInfo struct {
		i   int64
		rr  ranger.Ranger
		err error
	}
	rrch := make(chan rangerInfo, redundancy.GetTotal())
	for i := 0; i < len(remote.GetRemotePieces()); i++ {
		go func(i int) {
			cl := client.New(ctx, conns[i])
			rr, err := ranger.GRPCRanger(cl, remote.GetPieceId())
			rrch <- rangerInfo{remote.GetRemotePieces()[i].GetPieceNum(), rr, err}
		}(i)
	}
	for range conns {
		info := <-rrch
		if info.err != nil {
			// TODO don't fail just because of failure with only one node
			return info.err
		}
		rrs[int(info.i)] = info.rr
	}
	rr, err := eestream.Decode(context.Background(), rrs, es, 4*1024*1024)
	if err != nil {
		return err
	}
	rr, err = eestream.Transform(rr, decrypter)
	if err != nil {
		return err
	}
	rr, err = eestream.UnpadSlow(rr)
	if err != nil {
		return err
	}
	if length == -1 {
		length = rr.Size()
	}
	r := rr.Range(startOffset, length)
	_, err = io.Copy(writer, r)
	return err
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket, object string) (objInfo minio.ObjectInfo, err error) {
	defer func() {
		if err != nil {
			fmt.Printf("%+v", Error.Wrap(err))
		}
	}()

	/* making request to network state to get the piece IDs */
	/* create a unique fileID */
	netStateKey := []byte(*key)
	encryptedPath, err := paths.Encrypt([]string{bucket, object}, netStateKey)

	pointer, err := netstateGet(ctx, path.Join(encryptedPath...))
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	size, err := strconv.ParseInt(string(pointer.EncryptedUnencryptedSize), 10, 64)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	return minio.ObjectInfo{
		Bucket:      bucket,
		Name:        object,
		ModTime:     time.Date(2018, 6, 5, 17, 20, 32, 0, time.Local),
		Size:        size,
		IsDir:       false,
		ContentType: "video/mp4",
	}, nil
}

func (s *storjObjects) ListBuckets(ctx context.Context) (
	buckets []minio.BucketInfo, err error) {
	return s.getBuckets()
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result minio.ListObjectsInfo, err error) {
	return s.getFiles(bucket)
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	return s.addBucket(bucket)
}

/* Integration stuff with Network state */
var (
	netstateAddress = flag.String("netstate_addr", "35.202.58.176:4242", "address")
)

func netstatePut(ctx context.Context, path string, k, m, o, n int, size int64, pieceID string, nodes []*pb.RemotePiece) (err error) {
	defer mon.Task()(&ctx)(&err)

	conn, err := grpc.Dial(*netstateAddress, grpc.WithInsecure())
	if err != nil {
		zap.L().Error("Failed to dial: ", zap.Error(err))
		return Error.Wrap(err)
	}

	client := pb.NewNetStateClient(conn)

	//zap.L().Debug(fmt.Sprintf("client dialed port %s", netstateAddress))

	// Example pointer paths to put
	//pr1 passes with api creds
	pr1 := pb.PutRequest{
		Path: []byte(path), //key
		Pointer: &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					Type:             pb.RedundancyScheme_RS,
					MinReq:           int64(k),
					Total:            int64(n),
					RepairThreshold:  int64(m),
					SuccessThreshold: int64(o),
				},
				PieceId:      pieceID, //piece id (with frame #)
				RemotePieces: nodes,   //list of node ids
			},
			EncryptedUnencryptedSize: []byte(fmt.Sprint(size)),
		},
		APIKey: []byte("abc123"),
	}

	// Example Puts
	// puts passes api creds
	_, err = client.Put(ctx, &pr1)
	if err != nil || status.Code(err) == codes.Internal {
		zap.L().Error("failed to put", zap.Error(err))
		return Error.Wrap(err)
	}
	return nil
}

func prettyprint(val interface{}) {
	bytes, err := json.MarshalIndent(val, "", " ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(bytes))
}

//encryptFile encrypts the uploaded files
func encryptFile(ctx context.Context, data *hash.Reader, bucket, object string) (err error) {
	defer mon.Task()(&ctx)(&err)

	fc, err := infectious.NewFEC(*rsk, *rsn)
	if err != nil {
		return err
	}

	/* create a unique fileID */
	netStateKey := []byte(*key)
	encryptedPath, err := paths.Encrypt([]string{bucket, object}, netStateKey)
	fmt.Println("encrypted path ", encryptedPath)
	decryptedPath, err := paths.Decrypt(encryptedPath, netStateKey)
	fmt.Println("decrypted path ", decryptedPath)

	es := eestream.NewRSScheme(fc, *pieceBlockSize)
	encKey := sha256.Sum256([]byte(*key))
	var firstNonce [12]byte
	encrypter, err := eestream.NewAESGCMEncrypter(
		&encKey, &firstNonce, es.DecodedBlockSize())
	if err != nil {
		return err
	}
	readers, err := eestream.EncodeReader(context.Background(), eestream.TransformReader(
		eestream.PadReader(ioutil.NopCloser(data), encrypter.InBlockSize()), encrypter, 0),
		es, *rsm, *rso, 4*1024*1024)
	if err != nil {
		return Error.Wrap(err)
	}

	/* integrating with DHT for uploading to get the farmer's IP address */
	addr := "bootstrap.storj.io:7070"
	c, err := overlay.NewClient(&addr, grpc.WithInsecure())
	if err != nil {
		return Error.Wrap(err)
	}

	r, err := c.FindStorageNodes(context.Background(), &opb.FindStorageNodesRequest{})
	if err != nil {
		return Error.Wrap(err)
	}
	fmt.Printf("r %#v\n", r)
	pieceId := pstore.DetermineID()
	// r is your nodes
	var remotePieces []*pb.RemotePiece
	errs := make(chan error, len(readers))
	for i := range readers {
		remotePieces = append(remotePieces, &pb.RemotePiece{
			PieceNum: int64(i),
			NodeId:   r.Node[i].Id,
		})
		go func(i int) {
			conn, err := grpc.Dial(r.Node[i].Address.Address, grpc.WithInsecure())
			if err != nil {
				zap.S().Errorf("error dialing: %v\n", err)
				errs <- Error.Wrap(err)
				return
			}
			defer conn.Close()
			cl := client.New(ctx, conn)

			w, err := cl.StorePieceRequest(pieceId, 0)
			if err != nil {
				zap.S().Errorf("error req: %v\n", err)
				errs <- Error.Wrap(err)
				return
			}
			_, err = io.Copy(w, readers[i])
			// Make sure writer is closed to avoid losing last bytes
			if err != nil {
				w.Close()
				zap.S().Errorf("error copy: %v\n", err)
				errs <- Error.Wrap(err)
				return
			}
			err = w.Close()
			if err != io.EOF {
				zap.S().Errorf("error close: %v\n", err)
				errs <- Error.Wrap(err)
				return
			}
			errs <- nil
		}(i)
	}
	var errbucket []error
	for range readers {
		err := <-errs
		if err != nil {
			errbucket = append(errbucket, err)
		}
	}
	if len(readers)-len(errbucket) < *rsm {
		for _, err := range errbucket {
			fmt.Printf("%+v\n", err)
		}
		return errbucket[0]
	}
	/* integrate with Network State */
	// initialize the client
	return Error.Wrap(netstatePut(ctx, path.Join(encryptedPath...), es.RequiredCount(), *rsm, *rso, es.TotalCount(), data.Size(), pieceId, remotePieces))
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo minio.ObjectInfo,
	err error) {
	defer mon.Task()(&ctx)(&err)
	err = encryptFile(ctx, data, bucket, object)
	if err == nil {
		s.uploadFile(bucket, object, data.Size(), metadata)
	}
	return minio.ObjectInfo{
		Name:    object,
		Bucket:  bucket,
		ModTime: time.Now(),
		Size:    data.Size(),
		ETag:    minio.GenETag(),
	}, err
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) minio.StorageInfo {
	return minio.StorageInfo{}
}
