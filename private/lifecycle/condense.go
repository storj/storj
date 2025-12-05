// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package lifecycle

import (
	"bufio"
	"bytes"
)

func condenseStack(buf []byte) (out []byte) {
	// if the stack parsing below fails for any reason then just return the
	// original stack. it being too big is better than nothing.
	defer func() {
		if recover() != nil {
			out = buf
		}
	}()

	lines := bufio.NewScanner(bytes.NewReader(buf))
	skipNext := false

	for lines.Scan() {
		if skipNext {
			skipNext = false
			continue
		}

		switch line := lines.Bytes(); {
		case len(line) == 0:
			out = append(out, '\n')

		case bytes.HasPrefix(line, []byte(`goroutine `)):
			const gi = len("goroutine ")
			line = line[:gi+bytes.IndexByte(line[gi:], ' ')]
			out = append(out, line...)
			out = append(out, '\n')

		case line[0] == '\t':
			line = line[bytes.LastIndexByte(line, ':')+1:]
			if n := bytes.IndexByte(line, ' '); n >= 0 {
				line = line[:n]
			}
			out = append(out, line...)
			out = append(out, '\n')

		case bytes.HasPrefix(line, []byte(`created by`)):
			skipNext = true

		default:
			line = line[:bytes.LastIndexByte(line, '(')]
			out = append(out, '\t')
			out = append(out, line...)
			out = append(out, ':')
		}
	}

	if lines.Err() != nil {
		return buf
	}

	return out
}
