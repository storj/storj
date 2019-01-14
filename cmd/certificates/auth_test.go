package main

import (
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"path/filepath"
	"storj.io/storj/internal/testcontext"
	"strconv"
	"testing"
)

type cmdEnum int

type command string

const (
	cmdCertificates = cmdEnum(iota)
)

func TestCmdCreateAuth(t *testing.T) {
	assert := assert.New(t)
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	commands, err := tempBuild(ctx, []cmdEnum{cmdCertificates})
	if !assert.NoError(err) {
		t.Fatal(err)
	}

	// `certificates auth create 1 user@example.com`
	cases := map[string]int{
		"one@example.com": 1,
		"two@example.com": 2,
		"ten@example.com": 10,
	}

	for userID, count := range cases {
		err := commands[cmdCertificates].Run("auth", "create", strconv.Itoa(count), userID)
		if !assert.NoError(err) {
			t.Fatal(err)
		}
	}

}

func tempBuild(ctx *testcontext.Context, cmdNames []cmdEnum) (cmds map[cmdEnum]command, err error) {
	tmp := ctx.Dir("build")
	cmds = make(map[cmdEnum]command)

	for _, c := range cmdNames {
		cmdPath := filepath.Join(tmp, c.String())
		build := exec.Command("go", "build", "-o", cmdPath)
		build.Stdout = os.Stdout
		build.Stderr = os.Stderr

		if err = build.Run(); err != nil {
			return nil, err
		}

		cmds[c] = command(cmdPath)
	}

	return cmds, err
}

func (c command) Run(args ...string) error {
	cmd := exec.Command(string(c), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func (c cmdEnum) String() string {
	switch c {
	case 0:
		return "certificates"
	default:
		panic("unknown command")
	}
}
