package commands

import (
  "errors"
  "fmt"
  "log"
  "os"
  "os/user"
	"path"

  "github.com/spf13/viper"
  "github.com/urfave/cli"
)

var Delete = cli.Command{
  Name:      "delete",
  Aliases:   []string{"d"},
  Usage:     "delete farmer node",
  ArgsUsage: "[id]",
  Action: deleteNode,
}

// deleteNode deletes node config and all data stored on node by node ID
func deleteNode(c *cli.Context) error {
  nodeID := c.Args().Get(0)
  if nodeID == "" {
    return errors.New("no id specified")
  }

  usr, err := user.Current()
  if err != nil {
    log.Fatalf(err.Error())
  }

  configPath := path.Join(usr.HomeDir, ".storj/")
  config := path.Join(configPath, nodeID + ".yaml")

  if _, err = os.Stat(config); os.IsNotExist(err) {
    return errors.New("Invalid node id. Config file does not exist")
  }

  viper.SetConfigName(nodeID)
  viper.AddConfigPath(configPath)
  if err := viper.ReadInConfig(); err != nil {
    log.Fatalf(err.Error())
  }

  // get folder for stored data
  piecestoreDir := viper.GetString("piecestore.dir")
  piecestoreDir = path.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeID))

  // remove all folders and files stored on node
  if err = os.RemoveAll(piecestoreDir); err != nil {
    log.Fatalf(err.Error())
  }

  // delete node config
  err = os.Remove(config)
  if err != nil {
    log.Fatalf(err.Error())
  }

  log.Printf("Deleted node: %s", nodeID)

  return nil
}
