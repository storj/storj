package commands

import (
  "crypto/rand"
  "errors"
  "fmt"
  "log"
	"os"
	"os/user"
	"path"


	_ "github.com/mattn/go-sqlite3"
  "github.com/mr-tron/base58/base58"
  "github.com/spf13/viper"
	"github.com/urfave/cli"
)

var kadhost string
var kadport string
var kadlistenport string
var pshost string
var psport string
var dir string

var Create = cli.Command{
    Name:      "create",
    Aliases:   []string{"c"},
    Usage:     "create farmer node",
    ArgsUsage: "",
    Flags: []cli.Flag{
      cli.StringFlag{Name: "pieceStoreHost", Usage: "Farmer's public ip/host", Destination: &pshost},
      cli.StringFlag{Name: "pieceStorePort", Usage: "`port` where piece store data is accessed", Destination: &psport},
      cli.StringFlag{Name: "kademliaPort", Usage: "Kademlia server `host`", Destination: &kadport},
      cli.StringFlag{Name: "kademliaHost", Usage: "Kademlia server `host`", Destination: &kadhost},
      cli.StringFlag{Name: "kademliaListenPort", Usage: "Kademlia server `host`", Destination: &kadlistenport},
      cli.StringFlag{Name: "dir", Usage: "`dir` of drive being shared", Destination: &dir},
    },
    Action: createNode,
}

// createNode creates a new farmer node
func createNode(c *cli.Context) error {
  nodeID := newID()

  usr, err := user.Current()
  if err != nil {
    return err
  }

  viper.SetDefault("piecestore.host", "127.0.0.1")
  viper.SetDefault("piecestore.port", "7777")
  viper.SetDefault("piecestore.dir", usr.HomeDir)
  viper.SetDefault("piecestore.id", nodeID)
  viper.SetDefault("kademlia.host", "bootstrap.storj.io")
  viper.SetDefault("kademlia.port", "8080")
  viper.SetDefault("kademlia.listen.port", "7776")

  viper.SetConfigName(nodeID)
  viper.SetConfigType("yaml")

  configPath := path.Join(usr.HomeDir, ".storj/")
  if err = os.MkdirAll(configPath, 0700); err != nil {
    return err
  }

  viper.AddConfigPath(configPath)

  fullPath := path.Join(configPath, fmt.Sprintf("%s.yaml", nodeID))
  _, err = os.Stat(fullPath)
  if os.IsExist(err) {
    if err != nil {
      return errors.New("config already exists")
    }
    return err
  }

  // Create empty file at configPath
  _, err = os.Create(fullPath)
  if err != nil {
    return err
  }

  if pshost != "" {
    viper.Set("piecestore.host", pshost)
  }
  if psport != "" {
    viper.Set("piecestore.port", psport)
  }
  if dir != "" {
    viper.Set("piecestore.dir", dir)
  }
  if kadhost != "" {
    viper.Set("kademlia.host", kadhost)
  }
  if kadport != "" {
    viper.Set("kademlia.port", kadport)
  }
  if kadlistenport != "" {
    viper.Set("kademlia.listen.port", kadlistenport)
  }

  if err := viper.WriteConfig(); err != nil {
    return err
  }

  path := viper.ConfigFileUsed()

  log.Printf("Config: %s\n", path)
  log.Printf("ID: %s\n", nodeID)

  return nil
}

func newID() string {
	b := make([]byte, 32)

	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}

	encoding := base58.Encode(b)

	return encoding[:20]
}
