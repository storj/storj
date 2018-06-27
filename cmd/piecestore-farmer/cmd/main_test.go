// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
  "fmt"
  "os"
  "path/filepath"
  "testing"

  homedir "github.com/mitchellh/go-homedir"
  "github.com/spf13/viper"
  "github.com/stretchr/testify/assert"
)

func TestCreate(t *testing.T) {
  home, err := homedir.Dir()
  if err != nil {
    t.Error("Error: ", err)
  }

  tests := []struct{
    it string
    expectedConfig Config
    args []string
    err string
  } {
      {
        it: "should successfully write config with default values",
        expectedConfig: Config{
          NodeID:        "",
        	PsHost:        "127.0.0.1",
        	PsPort:        "7777",
        	KadListenPort: "7776",
        	KadPort:       "8080",
        	KadHost:       "bootstrap.storj.io",
        	PieceStoreDir: home,
        },
        args: []string{
          "create",
        },
        err: "",
      },
      {
        it: "should successfully write config with flag values",
        expectedConfig: Config{
          NodeID:        "",
        	PsHost:        "123.4.5.6",
        	PsPort:        "1234",
        	KadListenPort: "4321",
        	KadPort:       "4444",
        	KadHost:       "hack@theplanet.com",
        	PieceStoreDir: home,
        },
        args: []string{
          "create",
          fmt.Sprintf("--pieceStoreHost=%s", "123.4.5.6"),
          fmt.Sprintf("--pieceStorePort=%s", "1234"),
          fmt.Sprintf("--kademliaListenPort=%s", "4321"),
          fmt.Sprintf("--kademliaPort=%s", "4444"),
          fmt.Sprintf("--kademliaHost=%s", "hack@theplanet.com"),
        },
        err: "",
      },
      {
        it: "should err when a config with identical ID already exists",
        expectedConfig: Config{
          NodeID:        "",
        	PsHost:        "123.4.5.6",
        	PsPort:        "1234",
        	KadListenPort: "4321",
        	KadPort:       "4444",
        	KadHost:       "hack@theplanet.com",
        	PieceStoreDir: home,
        },
        args: []string{
          "create",
        },
        err: "Config already exists",
      },
  }

  for _, tt := range tests {
    t.Run(tt.it, func(t *testing.T) {
      assert := assert.New(t)

      RootCmd.SetArgs(tt.args)

      path := filepath.Join(home, ".storj", viper.GetString("piecestore.id") + ".yaml")
      defer os.Remove(path)

      if tt.err == "Config already exists" {
        _, err := os.Create(path)
        if err != nil {
          t.Errorf(err.Error())
        }
      }

      err := RootCmd.Execute()
      if tt.err != "" {
        if err != nil {
          assert.Equal(tt.err, err.Error())
        }
      } else if err != nil {
        t.Error(err.Error())
      }

      viper.SetConfigFile(path)

      err = viper.ReadInConfig()
      if err != nil {
        t.Errorf(err.Error())
      }

      tt.expectedConfig.NodeID = viper.GetString("piecestore.id")

      assert.Equal(tt.expectedConfig, GetConfigValues())
    })
  }
}
// func TestStart(t *testing.T) {
//
// }
//
// func TestDelete(t *testing.T) {
//
// }

func TestMain(m *testing.M) {
  m.Run()
}
