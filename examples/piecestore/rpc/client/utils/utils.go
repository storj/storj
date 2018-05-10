package utils // import "github.com/cam-a/storj-client"

import (
	"crypto/md5"
	"fmt"
	"io"
  "os"
)

// Get the hash for a section of data
func DetermineHash(f *os.File, offset int64, length int64) (string, error) {
	h := md5.New()

	fSection := io.NewSectionReader(f, offset, length)
	if _, err := io.Copy(h, fSection); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
