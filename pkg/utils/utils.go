// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package utils

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strings"
	"time"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/pb"
)

// GetBytes transforms an empty interface type into a byte slice
func GetBytes(key interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(key)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseURL extracts database parameters from a string as a URL
func ParseURL(s string) (string, string, error) {
	parts := strings.SplitAfterN(s, "://", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("Could not parse DB URL %s", s)
	}
	return parts[0], parts[1], nil
}

// CombineErrors combines multiple errors to a single error
func CombineErrors(errs ...error) error {
	var errlist combinedError
	for _, err := range errs {
		if err != nil {
			errlist = append(errlist, err)
		}
	}
	if len(errlist) == 0 {
		return nil
	} else if len(errlist) == 1 {
		return errlist[0]
	}
	return errlist
}

type combinedError []error

func (errs combinedError) Cause() error {
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (errs combinedError) Error() string {
	if len(errs) > 0 {
		limit := 5
		if len(errs) < limit {
			limit = len(errs)
		}
		allErrors := errs[0].Error()
		for _, err := range errs[1:limit] {
			allErrors += "\n" + err.Error()
		}
		return allErrors
	}
	return ""
}

// CollectErrors returns first error from channel and all errors that happen within duration
func CollectErrors(errch chan error, duration time.Duration) error {
	errch = discardNil(errch)
	errs := []error{<-errch}
	timeout := time.After(duration)
	for {
		select {
		case err := <-errch:
			errs = append(errs, err)
		case <-timeout:
			return CombineErrors(errs...)
		}
	}
}

// discard nil errors that are returned from services
func discardNil(ch chan error) chan error {
	r := make(chan error)
	go func() {
		for err := range ch {
			if err == nil {
				continue
			}
			r <- err
		}
		close(r)
	}()
	return r
}

// NodeIDsToLookupRequests ...
func NodeIDsToLookupRequests(nodeIDs []dht.NodeID) *pb.LookupRequests {
	var rq []*pb.LookupRequest
	for _, v := range nodeIDs {
		r := &pb.LookupRequest{NodeID: v.String()}
		rq = append(rq, r)
	}
	return &pb.LookupRequests{LookupRequest: rq}
}

// LookupResponsesToNodes ...
func LookupResponsesToNodes(responses *pb.LookupResponses) []*pb.Node {
	var nodes []*pb.Node
	for _, v := range responses.LookupResponse {
		n := v.Node
		nodes = append(nodes, n)
	}
	return nodes
}
