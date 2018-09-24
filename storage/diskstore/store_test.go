package diskstore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"

	"storj.io/storj/storage"
	"storj.io/storj/storage/diskstore"
)

func TestStoreLoad(t *testing.T) {
	const blobSize = 8 << 10
	const repeatCount = 16

	ctx := context.Background()

	dir, err := ioutil.TempDir("", "diskstore")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatal(err)
		}
	}()

	disk, err := diskstore.NewDisk(dir)
	if err != nil {
		t.Fatal(err)
	}

	store := diskstore.New(disk)

	data := make([]byte, blobSize)
	temp := make([]byte, len(data))
	_, _ = rand.Read(data)

	refs := map[storage.BlobRef]bool{}

	// store without size
	for i := 0; i < repeatCount; i++ {
		ref, err := store.Store(ctx, bytes.NewReader(data), -1)
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with size
	for i := 0; i < repeatCount; i++ {
		ref, err := store.Store(ctx, bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with larger size
	{
		ref, err := store.Store(ctx, bytes.NewReader(data), int64(len(data))*2)
		if err != nil {
			t.Fatal(err)
		}
		if refs[ref] {
			t.Fatal("duplicate ref received")
		}
		refs[ref] = true
	}

	// store with error
	{
		_, err := store.Store(ctx, &errorReader{}, int64(len(data)))
		if err == nil {
			t.Fatal("expected store error")
		}
	}

	// try reading all the blobs
	for ref := range refs {
		reader, err := store.Load(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}

		_, err = io.ReadFull(reader, temp)
		if err != nil {
			t.Fatal(err)
		}

		err = reader.Close()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(data, temp) {
			t.Fatal("data mismatch")
		}
	}

	// delete the blobs
	for ref := range refs {
		err := store.Delete(ctx, ref)
		if err != nil {
			t.Fatal(err)
		}
	}

	// try reading all the blobs
	for ref := range refs {
		_, err := store.Load(ctx, ref)
		if err == nil {
			t.Fatal("expected error when loading invalid ref")
		}
	}
}

type errorReader struct{}

func (errorReader *errorReader) Read(data []byte) (n int, err error) {
	return 0, errors.New("internal-error")
}
