package miniogw

import (
	"context"
	"errors"
	"io"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	minio "github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/hash"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storage/objects"
)

type MultipartUploads struct {
	mu      sync.RWMutex
	id      int
	pending map[string]*MultipartUpload
}

func NewMultipartUploads() *MultipartUploads {
	return &MultipartUploads{
		pending: map[string]*MultipartUpload{},
	}
}

type MultipartUpload struct {
	Bucket   string
	Object   string
	Metadata map[string]string
	Done     chan (*MultipartUploadResult)
	Stream   *MultipartStream
}

func (upload *MultipartUpload) Fail(err error) {
	upload.Done <- &MultipartUploadResult{Error: err}
	close(upload.Done)
}

func (upload *MultipartUpload) Complete(info minio.ObjectInfo) {
	upload.Done <- &MultipartUploadResult{Info: info}
	close(upload.Done)
}

type MultipartUploadResult struct {
	Error error
	Info  minio.ObjectInfo
}

func NewMultipartUpload(bucket, object string, metadata map[string]string) *MultipartUpload {
	upload := &MultipartUpload{
		Bucket:   bucket,
		Object:   object,
		Metadata: metadata,
		Done:     make(chan *MultipartUploadResult, 1),
		Stream:   &MultipartStream{},
	}
	upload.Stream.blocked.L = &upload.Stream.mu
	upload.Stream.nextId = 1
	return upload
}

type MultipartStream struct {
	mu         sync.Mutex
	blocked    sync.Cond
	err        error
	closed     bool
	nextId     int
	nextNumber int
	parts      []*StreamPart
}

type StreamPart struct {
	Number int
	ID     int
	Size   int64
	Reader *hash.Reader
	Done   chan error
}

func (stream *MultipartStream) Abort(err error) {
	stream.blocked.L.Lock()
	defer stream.blocked.L.Unlock()

	stream.err = err
	stream.closed = true

	for _, part := range stream.parts {
		part.Done <- err
		close(part.Done)
	}
	stream.parts = nil

	stream.blocked.Broadcast()
}

func (stream *MultipartStream) Close() {
	stream.blocked.L.Lock()
	defer stream.blocked.L.Unlock()

	stream.closed = true
	stream.blocked.Broadcast()
}

func (stream *MultipartStream) Read(data []byte) (n int, err error) {
	var part *StreamPart
	stream.blocked.L.Lock()
	for {
		if stream.err != nil {
			stream.blocked.L.Unlock()
			return 0, err
		}
		if len(stream.parts) > 0 && stream.nextId == stream.parts[0].ID {
			part = stream.parts[0]
			break
		}

		if stream.closed {
			stream.blocked.L.Unlock()
			return 0, io.EOF
		}

		// blocked for more parts
		stream.blocked.Wait()
	}
	stream.blocked.L.Unlock()

	n, err = part.Reader.Read(data)
	atomic.AddInt64(&part.Size, int64(n))

	if err == io.EOF {
		err = nil
		stream.blocked.L.Lock()
		stream.parts = stream.parts[1:]
		stream.nextId++
		stream.blocked.L.Unlock()

		close(part.Done)
	} else if err != nil {
		stream.Abort(err)
	}

	return n, err
}

func (stream *MultipartStream) AddPart(partID int, data *hash.Reader) (*StreamPart, error) {
	stream.blocked.L.Lock()
	defer stream.blocked.L.Unlock()

	stream.nextNumber++
	part := &StreamPart{
		Number: stream.nextNumber - 1,
		ID:     partID,
		Size:   0,
		Reader: data,
		Done:   make(chan error, 1),
	}

	// TODO: check for duplicate

	stream.parts = append(stream.parts, part)
	sort.Slice(stream.parts, func(i, k int) bool {
		return stream.parts[i].ID < stream.parts[k].ID
	})

	stream.blocked.Broadcast()

	return part, nil
}

func (uploads *MultipartUploads) lookupUpload(bucket, object, uploadID string) (*MultipartUpload, error) {
	uploads.mu.Lock()
	defer uploads.mu.Unlock()

	upload, ok := uploads.pending[uploadID]
	if !ok {
		return nil, errors.New("pending upload " + uploadID + " missing")
	}

	if upload.Bucket != bucket || upload.Object != object {
		return nil, errors.New("pending upload " + uploadID + " bucket/object name mismatch")
	}

	return upload, nil
}

func (uploads *MultipartUploads) lookupAndRemoveUpload(bucket, object, uploadID string) (*MultipartUpload, error) {
	uploads.mu.RLock()
	defer uploads.mu.RUnlock()

	upload, ok := uploads.pending[uploadID]
	if !ok {
		return nil, errors.New("pending upload " + uploadID + " missing")
	}

	if upload.Bucket != bucket || upload.Object != object {
		return nil, errors.New("pending upload " + uploadID + " bucket/object name mismatch")
	}

	delete(uploads.pending, uploadID)

	return upload, nil
}

func (s *storjObjects) NewMultipartUpload(ctx context.Context, bucket, object string, metadata map[string]string) (uploadID string, err error) {
	uploads := s.storj.Multipart
	uploads.mu.Lock()
	defer uploads.mu.Unlock()

	for _, upload := range uploads.pending {
		if upload.Bucket == bucket && upload.Object == object {
			return "", errors.New("duplicate upload")
		}
	}

	uploads.id++
	uploadID = "Upload" + strconv.Itoa(uploads.id)

	upload := NewMultipartUpload(bucket, object, metadata)
	uploads.pending[uploadID] = upload

	objectStore, err := s.storj.bs.GetObjectStore(ctx, bucket)
	if err != nil {
		return "", err
	}

	go func() {
		// setting zero value means the object never expires
		expTime := time.Time{}

		tempContType := metadata["content-type"]
		delete(metadata, "content-type")
		//metadata serialized
		serMetaInfo := objects.SerializableMeta{
			ContentType: tempContType,
			UserDefined: metadata,
		}

		result, err := objectStore.Put(ctx, paths.New(object), upload.Stream, serMetaInfo, expTime)

		uploads.mu.Lock()
		delete(uploads.pending, uploadID)
		uploads.mu.Unlock()

		if err != nil {
			upload.Fail(err)
		} else {
			upload.Complete(minio.ObjectInfo{
				Name:        object,
				Bucket:      bucket,
				ModTime:     result.Modified,
				Size:        result.Size,
				ETag:        result.Checksum,
				ContentType: result.ContentType,
				UserDefined: result.UserDefined,
			})
		}
	}()

	return uploadID, nil
}

func (s *storjObjects) PutObjectPart(ctx context.Context, bucket, object, uploadID string, partID int, data *hash.Reader) (info minio.PartInfo, err error) {
	uploads := s.storj.Multipart

	upload, err := uploads.lookupUpload(bucket, object, uploadID)
	if err != nil {
		return minio.PartInfo{}, err
	}

	part, err := upload.Stream.AddPart(partID, data)
	if err != nil {
		return minio.PartInfo{}, err
	}

	err = <-part.Done
	if err != nil {
		return minio.PartInfo{}, err
	}

	return minio.PartInfo{
		PartNumber:   part.Number,
		LastModified: time.Now(),
		ETag:         data.SHA256HexString(),
		Size:         atomic.LoadInt64(&part.Size),
	}, nil
}

func (s *storjObjects) AbortMultipartUpload(ctx context.Context, bucket, object, uploadID string) error {
	uploads := s.storj.Multipart

	upload, err := uploads.lookupAndRemoveUpload(bucket, object, uploadID)
	if err != nil {
		return err
	}

	errAbort := errors.New("abort")
	upload.Stream.Abort(errAbort)
	r := <-upload.Done
	if r.Error != errAbort {
		return r.Error
	}
	return nil
}

func (s *storjObjects) CompleteMultipartUpload(ctx context.Context, bucket, object, uploadID string, uploadedParts []minio.CompletePart) (objInfo minio.ObjectInfo, err error) {
	uploads := s.storj.Multipart
	upload, err := uploads.lookupAndRemoveUpload(bucket, object, uploadID)
	if err != nil {
		return minio.ObjectInfo{}, err
	}

	upload.Stream.Close()

	result := <-upload.Done
	return result.Info, result.Error
}

// func (s *storjObjects) ListMultipartUploads(ctx context.Context, bucket, prefix, keyMarker, uploadIDMarker, delimiter string, maxUploads int) (result minio.ListMultipartsInfo, err error) {
// 	return
// }
//
// func (s *storjObjects) ListObjectParts(ctx context.Context, bucket, object, uploadID string, partNumberMarker int, maxParts int) (result minio.ListPartsInfo, err error) {
// 	return
// }
//
// func (s *storjObjects) CopyObjectPart(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, uploadID string, partID int, startOffset int64, length int64, srcInfo minio.ObjectInfo) (info minio.PartInfo, err error) {
// 	return
// }
