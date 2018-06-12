package commands

import (
  "errors"
  "fmt"
	"log"
	"net"
	"os"
	"os/user"
	"path"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
  "golang.org/x/net/context"
  "google.golang.org/grpc"


  "storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/piecestore/rpc/server"
	"storj.io/storj/pkg/piecestore/rpc/server/ttl"
	pb "storj.io/storj/protos/piecestore"
  proto "storj.io/storj/protos/overlay"
)

var Start = cli.Command{
    Name:      "start",
    Aliases:   []string{"s"},
    Usage:     "start farmer node",
    ArgsUsage: "[id]",
    Action: startNode,
}

// startNode starts farmer node by node ID
func startNode(c *cli.Context) error {

  if c.Args().Get(0) == "" {
    return errors.New("no id specified")
  }

  usr, err := user.Current()
  if err != nil {
    log.Fatalf(err.Error())
  }

  configPath := path.Join(usr.HomeDir, ".storj/")
  viper.AddConfigPath(configPath)
  viper.SetConfigName(c.Args().Get(0))
  viper.SetConfigType("yaml")
  if err := viper.ReadInConfig(); err != nil {
    log.Fatalf(err.Error())
  }

  nodeid := viper.GetString("piecestore.id")
  pshost = viper.GetString("piecestore.host")
  psport = viper.GetString("piecestore.port")
  kadlistenport = viper.GetString("kademlia.listen.port")
  kadport = viper.GetString("kademlia.port")
  kadhost = viper.GetString("kademlia.host")
  piecestoreDir := viper.GetString("piecestore.dir")
  dbPath := path.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeid), "/ttl-data.db")
  dataDir := path.Join(piecestoreDir, fmt.Sprintf("store-%s", nodeid), "/piece-store-data/")

  if err = os.MkdirAll(piecestoreDir, 0700); err != nil {
    log.Fatalf(err.Error())
  }

  _ = connectToKad(nodeid, pshost, kadlistenport, fmt.Sprintf("%s:%s", kadhost, kadport))

  fileInfo, err := os.Stat(piecestoreDir)
  if err != nil {
    log.Fatalf(err.Error())
  }
  if fileInfo.IsDir() != true {
    log.Fatalf("Error: %s is not a directory", piecestoreDir)
  }

  ttlDB, err := ttl.NewTTL(dbPath)
  if err != nil {
    log.Fatalf("failed to open DB")
  }

  // create a listener on TCP port
  lis, err := net.Listen("tcp", fmt.Sprintf(":%s", psport))
  if err != nil {
    log.Fatalf("failed to listen: %v", err)
  }
  defer lis.Close()

  // create a server instance
  s := server.Server{PieceStoreDir: dataDir, DB: ttlDB}

  // create a gRPC server object
  grpcServer := grpc.NewServer()

  // attach the api service to the server
  pb.RegisterPieceStoreRoutesServer(grpcServer, &s)

  // routinely check DB and delete expired entries
  go func() {
    err := s.DB.DBCleanup(dataDir)
    log.Fatalf("Error in DBCleanup: %v", err)
  }()

  // start the server
  if err := grpcServer.Serve(lis); err != nil {
    log.Fatalf("failed to serve: %s", err)
  }
  return nil
}

func connectToKad(id, ip, kadlistenport, kadaddress string) *kademlia.Kademlia {
	node := proto.Node{
		Id: string(id),
		Address: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP,
			Address:   kadaddress,
		},
	}

	kad, err := kademlia.NewKademlia([]proto.Node{node}, ip, kadlistenport)
	if err != nil {
		log.Fatalf("Failed to instantiate new Kademlia: %s", err.Error())
	}

	if err := kad.ListenAndServe(); err != nil {
		log.Fatalf("Failed to ListenAndServe on new Kademlia: %s", err.Error())
	}

	if err := kad.Bootstrap(context.Background()); err != nil {
		log.Fatalf("Failed to Bootstrap on new Kademlia: %s", err.Error())
	}

	return kad
}
