package main

import (
  "io/ioutil"
  "fmt"
  "log"
  "os"
  "regexp"

  "github.com/urfave/cli"
  "gopkg.in/yaml.v2"
)

type Config struct {
  IP string
  Port string
}

var config Config

func main() {
  app := cli.NewApp()

  app.Commands = []cli.Command{
    {
      Name:    "create",
      Aliases: []string{"c"},
      Usage:   "create config file",
      Action: func(c *cli.Context) error {
        var IP string
        var port string

        if matched, _ := regexp.MatchString(`^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$`, c.Args().Get(0)); matched != true {
          fmt.Println("Invalid IP")
          return nil
        } else {
            IP = c.Args().Get(0)
        }
        if matched, _ := regexp.MatchString(`^\d{2,6}$`, c.Args().Get(1)); matched != true {
    			fmt.Println("Invalid port")
          return nil
    		} else {
          port = c.Args().Get(1)
        }

        encoded, err := yaml.Marshal(&Config{IP, port})
        if err != nil {
          return err
        }

        file, err := os.Create("config.yaml")
        if err != nil {
          return err
        }

        _, err = file.Write(encoded)
        if err != nil {
          return err
        }

        return nil
      },
    },
    {
      Name:    "read",
      Aliases: []string{"r"},
      Usage:   "read config file",
      Action: func(c *cli.Context) error {
        if _, err := os.Stat("./config.yaml"); os.IsNotExist(err) {
          fmt.Println("Error: Config file does not exist")
          return err
        }

        data, err := ioutil.ReadFile("./config.yaml")
        if err != nil {
          return err
        }

        err = yaml.Unmarshal(data, &config)
        if err != nil {
          return err
        }

        fmt.Println("IP:", config.IP)
        fmt.Println("Port:", config.Port)
        return nil
      },
    },
  }
  err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
