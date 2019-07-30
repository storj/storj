// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package grpcmonkit

const (
	traceIDKey = "monkit-trace-id"
	spanIDKey  = "monkit-span-id"
)

func parseFullMethod(fullMethod string) (service, endpoint string) {
	for i, p := range fullMethod[1:] {
		if p == '/' {
			return fullMethod[:i+1], fullMethod[i+1:]
		}
	}
	return fullMethod, ""
}
