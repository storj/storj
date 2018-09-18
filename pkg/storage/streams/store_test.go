package streams

import (
	"context"
	"errors"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/paths"
	ranger "storj.io/storj/pkg/ranger"
	"storj.io/storj/pkg/storage/segments"
)

var ctx = context.Background()

type metaClosure = func(context.Context, paths.Path) (segments.Meta, error)
type getClosure = func(context.Context, paths.Path) (ranger.Ranger, segments.Meta, error)
type putClosure = func(context.Context, paths.Path, io.Reader, []byte, time.Time) (segments.Meta, error)
type deleteClosure = func(context.Context, paths.Path) error
type listClosure = func(context.Context, paths.Path, paths.Path, paths.Path, bool, int, uint32) ([]segments.ListItem, bool, error)

type segmentStub struct {
	mc metaClosure
	gc getClosure
	pc putClosure
	dc deleteClosure
	lc listClosure
}

func (s *segmentStub) Meta(ctx context.Context, path paths.Path) (meta segments.Meta, err error) {
	return s.mc(ctx, path)
}

func (s *segmentStub) Get(ctx context.Context, path paths.Path) (rr ranger.Ranger,
	meta segments.Meta, err error) {
	return s.gc(ctx, path)
}

func (s *segmentStub) Put(ctx context.Context, path paths.Path, data io.Reader, metadata []byte,
	expiration time.Time) (meta segments.Meta, err error) {
	return s.pc(ctx, path, data, metadata, expiration)
}
func (s *segmentStub) Delete(ctx context.Context, path paths.Path) (err error) {
	return s.dc(ctx, path)
}
func (s *segmentStub) List(ctx context.Context, prefix, startAfter, endBefore paths.Path,
	recursive bool, limit int, metaFlags uint32) (items []segments.ListItem,
	more bool, err error) {
	return s.lc(ctx, prefix, startAfter, endBefore, recursive, limit, metaFlags)
}

var segmentErrorString = "a segment error"
var protoErrorString = "proto: streams.MetaStreamInfo: wiretype end group for non-group"

func TestMeta(t *testing.T) {

	errorFn := func(ctx context.Context, path paths.Path) (segments.Meta, error) {
		return segments.Meta{}, errors.New(segmentErrorString)
	}
	emptyFn := func(ctx context.Context, path paths.Path) (segments.Meta, error) {
		return segments.Meta{
			Modified:   time.Time{},
			Expiration: time.Time{},
			Size:       0,
			Data:       []byte(""),
		}, nil
	}
	metaFn := func(ctx context.Context, path paths.Path) (segments.Meta, error) {
		return segments.Meta{
			Modified:   time.Now(),
			Expiration: time.Now(),
			Size:       10,
			Data:       []byte("data"),
		}, nil
	}

	metaSlice := []metaClosure{
		errorFn,
		emptyFn,
		metaFn,
	}

	pathSlice := []paths.Path{
		paths.New(""),
		paths.New("bucket"),
	}

	for i, closure := range metaSlice {
		for j, path := range pathSlice {
			errTag := fmt.Sprintf("Test case #%d, path #%d", i, j)

			segment := &segmentStub{}
			segment.mc = closure

			stream, err := NewStreamStore(segment, int64(10))
			if err != nil {
				assert.Empty(t, stream, errTag)
				t.Fatal(err)
			}

			meta, err := stream.Meta(ctx, path)
			if err != nil {
				if err.Error() == segmentErrorString {
					assert.Equal(t, segmentErrorString, err.Error(), errTag)
					continue
				}
				if err.Error() == protoErrorString {
					assert.Equal(t, protoErrorString, err.Error(), errTag)
					continue
				}

				assert.Empty(t, meta, errTag)
				t.Fatal(err)
			}
		}
	}
}
