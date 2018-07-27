// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package segments

import "io"

// PeekThresholdReader allows a check to see if the size of a given reader
// exceeds the maximum inline segment size or not.
type PeekThresholdReader struct {
	r              io.Reader
	thresholdBuf   []byte
	isLargerCalled bool
	readCalled     bool
}

// NewPeekThresholdReader creates a new instance of PeekThresholdReader
func NewPeekThresholdReader(r io.Reader) (pt *PeekThresholdReader) {
	return &PeekThresholdReader{r: r}
}

// Read initially reads bytes from the internal buffer, then continues
// reading from the wrapped data reader. The number of bytes read `n`
// is returned.
func (pt *PeekThresholdReader) Read(p []byte) (n int, err error) {
	pt.readCalled = true

	if len(pt.thresholdBuf) == 0 {
		return pt.r.Read(p)
	}

	n = copy(p, pt.thresholdBuf)
	pt.thresholdBuf = pt.thresholdBuf[n:]
	return n, nil
}

// IsLargerThan returns a bool to determine whether a reader's size
// is larger than the given threshold or not.
func (pt *PeekThresholdReader) IsLargerThan(thresholdSize int) (inline bool, err error) {
	if pt.isLargerCalled {
		return false, Error.New("IsLargerThan can't be called more than once")
	}
	if pt.readCalled {
		return false, Error.New("IsLargerThan can't be called after Read has been called")
	}
	buf := make([]byte, thresholdSize+1)
	n, err := io.ReadFull(pt.r, buf)
	if err != nil {
		// in this case, reader size is equal or less than the threshold
		if err == io.ErrUnexpectedEOF {
			return true, err
		}
		return false, err
	}
	pt.thresholdBuf = buf[0:n]
	if len(pt.thresholdBuf) <= thresholdSize {
		return true, nil
	}
	return false, nil
}
