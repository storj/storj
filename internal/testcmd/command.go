package testcmd

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"storj.io/storj/internal/testcontext"
)

type CmdEnum int
type Cmd struct {
	path    string
	Process *os.Process
	Stdout  *bytes.Buffer
	Stderr  *bytes.Buffer
	Stdin   *bytes.Buffer
}

const (
	CmdCertificates = CmdEnum(iota)
	CmdIdentity
	CmdStorageNode
)

func NewCmd(name string) *Cmd {
	return &Cmd{
		path:    name,
		Process: new(os.Process),
		Stdout:  new(bytes.Buffer),
		Stderr:  new(bytes.Buffer),
		Stdin:   new(bytes.Buffer),
	}
}

func Build(ctx *testcontext.Context, cmdNames ...CmdEnum) (cmdMap map[CmdEnum]*Cmd, err error) {
	cmdMap = make(map[CmdEnum]*Cmd)
	for _, c := range cmdNames {
		cmdPath := ctx.File("build", c.String())
		build := exec.Command(
			"go", "build", "-o", cmdPath,
			filepath.Join("storj.io", "storj", "cmd", c.String()),
		)
		//build.Stdout = os.Stdout
		//build.Stderr = os.Stderr

		if err = build.Run(); err != nil {
			return nil, err
		}

		cmdMap[c] = NewCmd(cmdPath)
	}

	return cmdMap, err
}

func (c Cmd) Run(args ...string) error {
	cmd := exec.Command(c.path, args...)
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin

	// TODO: currently `err == nil` if usage is printed (e.g.: "unknown flag", etc.)
	err := cmd.Run()
	if err != nil {
		log.Println(c.Stderr.String())
	}
	//c.Process = cmd.Process
	return err
}

func (c Cmd) Start(args ...string) error {
	cmd := exec.Command(c.path, args...)
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	cmd.Stdin = c.Stdin

	// TODO: currently `err == nil` if usage is printed (e.g.: "unknown flag", etc.)
	err := cmd.Start()
	if err != nil {
		log.Println(c.Stderr.String())
	}
	*c.Process = *cmd.Process
	return err
}

func (c Cmd) Kill() error {
	return c.Process.Kill()
}

func (c Cmd) UnreadStdout() (*bytes.Buffer, error) {
	stdout := new(bytes.Buffer)
	_, err := c.Stdout.WriteTo(stdout)
	if err != nil {
		return nil, err
	}
	return stdout, err
}

func (c Cmd) DrainStdout() error {
	if _, err := c.UnreadStdout(); err != nil {
		return err
	}
	return nil
}

func (c CmdEnum) String() string {
	switch c {
	case CmdCertificates:
		return "certificates"
	case CmdIdentity:
		return "identity"
	case CmdStorageNode:
		return "storagenode"
	default:
		panic("unknown command")
	}
}
