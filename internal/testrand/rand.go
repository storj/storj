package testrand

import (
	"math/rand"
	"testing"

	"storj.io/storj/internal/memory"
)

// Read reads pseudo-random data into data.
func Read(t *testing.T, data []byte) {
	t.Helper()
	src := rand.NewSource(rand.Int63())
	r := rand.New(src)
	_, err := r.Read(data)
	if err != nil {
		t.Fatal(err)
	}
}

// Data generates size amount of random data.
func Data(t *testing.T, size memory.Size) []byte {
	t.Helper()
	data := make([]byte, size.Int())
	Read(t, data)
	return data
}
