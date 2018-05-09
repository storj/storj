// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"context"

	"go.uber.org/zap"

	proto "storj.io/storj/protos/netstate"
	"storj.io/storj/storage/boltdb"
)

// NetState implements the network state RPC service
type NetState struct {
	DB     Client
	logger *zap.Logger
}

// Client interface allows more modular unit testing
// and makes it easier in the future to substitute
// db clients other than bolt
type Client interface {
	Put(boltdb.File) error
	Get([]byte) (boltdb.File, error)
	List() ([]string, error)
	Delete([]byte) error
}

// Put formats and hands off a file path to be saved to boltdb
func (n *NetState) Put(ctx context.Context, filepath *proto.FilePath) (*proto.PutResponse, error) {
	n.logger.Debug("entering NetState.Put(...)")

	file := boltdb.File{
		Path:  filepath.Path,
		Value: []byte(filepath.SmallValue),
	}

	if err := n.DB.Put(file); err != nil {
		n.logger.Error("err putting file", zap.Error(err))
		return nil, err
	}
	n.logger.Debug("the file was put to the db")

	return &proto.PutResponse{
		Confirmation: "success",
	}, nil
}

// Get formats and hands off a file path to get from boltdb
func (n *NetState) Get(ctx context.Context, filepath *proto.FilePath) (*proto.GetResponse, error) {
	n.logger.Debug("entering NetState.Get(...)")

	fileInfo, err := n.DB.Get([]byte(filepath.Path))
	if err != nil {
		n.logger.Error("err getting file", zap.Error(err))
		return nil, err
	}

	return &proto.GetResponse{
		Content: string(fileInfo.Value),
	}, nil
}

// List calls the bolt client's List function and returns all file paths
func (n *NetState) List(ctx context.Context, req *proto.ListRequest) (*proto.ListResponse, error) {
	n.logger.Debug("entering NetState.List(...)")

	filePaths, err := n.DB.List()
	if err != nil {
		n.logger.Error("err listing file paths", zap.Error(err))
		return nil, err
	}

	n.logger.Debug("file paths retrieved")
	return &proto.ListResponse{
		// filePaths is an array of strings
		Filepaths: filePaths,
	}, nil
}

// Delete formats and hands off a file path to delete from boltdb
func (n *NetState) Delete(ctx context.Context, filepath *proto.FilePath) (*proto.DeleteResponse, error) {
	n.logger.Debug("entering NetState.Delete(...)")

	err := n.DB.Delete([]byte(filepath.Path))
	if err != nil {
		n.logger.Error("err deleting file", zap.Error(err))
		return nil, err
	}
	n.logger.Debug("file deleted")
	return &proto.DeleteResponse{
		Confirmation: "success",
	}, nil
}
