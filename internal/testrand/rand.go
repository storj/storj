package testrand

import (
	"io"
	"math/rand"

	"storj.io/storj/internal/memory"
)

func Int63n(n int64) int64 {
	return rand.Int63n(n)
}

// Read reads pseudo-random data into data.
func Read(data []byte) {
	src := rand.NewSource(rand.Int63())
	r := rand.New(src)
	_, err := r.Read(data)
	if err != nil {
		panic(err) // should never happen
	}
}

// Bytes generates size amount of random data.
func Bytes(size memory.Size) []byte {
	data := make([]byte, size.Int())
	Read(data)
	return data
}

// Reader creates a new random data reader.
func Reader() io.Reader {
	return rand.New(rand.NewSource(rand.Int63()))
}
