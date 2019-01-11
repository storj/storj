// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pointerdb

import "storj.io/storj/pkg/pb"

type Service struct {
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Put(path string, pointer *pb.Pointer) (err error) {
	return nil
}

func (s *Service) Get(path string) (pointer *pb.Pointer, err error) {
	return nil, nil
}
