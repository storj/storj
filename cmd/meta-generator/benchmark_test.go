package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	expect "github.com/Netflix/go-expect"
	"github.com/google/goterm/term"
)

const (
	clusterPath = "/Users/bohdanbashynskyi/storj-cluster"
	timeout     = 2 * time.Minute
	label       = "benchmarks"
)

func setup() {
	apiKey = "15XZkUAXp3J8m93D9AcyTiYPdFPGsaMB2R1PqrUwLAoP6h2CDz5EUZ5WrGgERNxYjs9wRc4Rhwr95Qgcxj3gNgb5yr5cYSEmwWC2UdzaKTrDm31ivFeszaMbggvkqhoyHcwwvSnjN"
	satelliteAddress := "12whfK1EDvHJtajBiAUeajQLYcWqxcQmdYQU5zX5cCf6bAxfgu4@satellite-api:7777"

	//Uplink Setup
	c, err := expect.NewConsole(expect.WithStdout(os.Stdout))
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	cmd := exec.Command("uplink", "setup", "--force")
	cmd.Dir = clusterPath
	cmd.Stdin = c.Tty()
	cmd.Stdout = c.Tty()
	cmd.Stderr = c.Tty()

	err = cmd.Start()
	if err != nil {
		log.Fatal(err)
	}

	c.ExpectString("Enter name to import as [default: main]:")
	c.Send(label + "\n")
	c.ExpectString("Enter API key or Access grant:")
	c.Send(apiKey + "\n")
	c.ExpectString("Satellite address:")
	c.Send(satelliteAddress + "\n")
	c.ExpectString("Passphrase:")
	c.Send(label + "\n")
	c.ExpectString("Again:")
	c.Send(label + "\n")
	c.ExpectString("Would you like to disable encryption for object keys (allows lexicographical sorting of objects in listings)? (y/N):")
	c.Send("y\n")
	c.ExpectString("Would you like S3 backwards-compatible Gateway credentials? (y/N):")
	c.Send("y\n")
	fmt.Println(term.Greenf("Uplink setup done"))

	//Create bucket
	cmd = exec.Command("uplink", "mb", "sj://benchmarks")

	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "bucket already exists") {
		panic(err.Error())
	}
}

func clean() {
	//Remove bucket
	cmd := exec.Command("uplink", "rb", "sj://benchmarks")
	cmd.Dir = clusterPath

	_, err := cmd.CombinedOutput()
	if err != nil {
		panic(err.Error())
	}
}

func BenchmarkQueryByKey(b *testing.B) {
	setup()
	//clean()
}
