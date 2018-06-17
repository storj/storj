// Copyright © 2018 Storj Labs
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"crypto/rand"
	"path/filepath"

	"github.com/mr-tron/base58/base58"
	"github.com/spf13/viper"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
)

// Config stores values from a farmer node config file
type Config struct {
	NodeID        string
	PsHost        string
	PsPort        string
	KadListenPort string
	KadPort       string
	KadHost       string
	PieceStoreDir string
}

// NewLogger creates a new sugared logger
func NewLogger() (*zap.SugaredLogger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}

	defer logger.Sync()
	sugar := logger.Sugar()

	return sugar, nil
}

// NewID returns a 20 byte ID for a new farmer node
func NewID() (string, error) {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}

	encoding := base58.Encode(b)

	return encoding[:20], nil
}

// SetConfigPath sets and returns viper config directory and filepath
func SetConfigPath(dir, fileName string) (configDir, configFile string) {
	configDir = filepath.Join(dir, "/.storj")
	configFile = filepath.Join(configDir, fileName+".yaml")

	viper.AddConfigPath(configDir)
	viper.SetConfigName(fileName)
	viper.SetConfigType("yaml")

	return configDir, configFile
}

// GetConfigValues returns a struct with config file values
func GetConfigValues() Config {
	nodeID := viper.GetString("piecestore.id")
	psHost := viper.GetString("piecestore.host")
	psPort := viper.GetString("piecestore.port")
	kadListenPort := viper.GetString("kademlia.listen.port")
	kadPort := viper.GetString("kademlia.port")
	kadHost := viper.GetString("kademlia.host")
	pieceStoreDir := viper.GetString("piecestore.dir")

	config := Config{
		NodeID:        nodeID,
		PsHost:        psHost,
		PsPort:        psPort,
		KadListenPort: kadListenPort,
		KadPort:       kadPort,
		KadHost:       kadHost,
		PieceStoreDir: pieceStoreDir,
	}

	return config
}

// ConnectToKad joins the Kademlia network
func ConnectToKad(ctx context.Context, id, ip, kadListenPort, kadAddress string) (*kademlia.Kademlia, error) {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadAddress,
		},
	}

	kad, err := kademlia.NewKademlia([]proto.Node{node}, ip, kadListenPort)
	if err != nil {
		return nil, errs.New("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		return nil, errs.New("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(ctx); err != nil {
		return nil, errs.New("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad, nil
}
