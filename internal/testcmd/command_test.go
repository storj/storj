package testcmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"

	"storj.io/storj/internal/testcontext"
)

func TestBuild(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	cmds, err := Build(ctx, CmdIdentity, CmdStorageNode)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	assert.Len(t, cmds, 2)
}

func TestCmd_Run(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	path := ctx.File("TestCmd_Run", "testfile")
	_, err := os.Stat(path)
	if !assert.True(t, os.IsNotExist(err)) {
		t.Fatal("expected test file to not exist")
	}

	cmd := NewCmd("touch")
	err = cmd.Run(path)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	_, err = os.Stat(path)
	assert.NoError(t, err)
}

// TODO: test Run with error
// TODO: test Start with error
// TODO: test Kill with error?

func TestCmd_Start(t *testing.T) {
	cmd := NewCmd("echo")
	err := cmd.Start()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	err = cmd.Process.Kill()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
}

func TestCmd_Kill(t *testing.T) {
	cmd := exec.Command("echo")
	err := cmd.Start()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	err = cmd.Process.Kill()
	assert.NoError(t, err)
}

func TestCmdEnum_String(t *testing.T) {
	_, callerPath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller information available")
	}

	storjPath, err := filepath.Abs(filepath.Join(filepath.Dir(callerPath), "..", ".."))
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	pkgName := CmdIdentity.String()
	pkgPath := filepath.Join(storjPath, "cmd", pkgName)
	_, err = os.Stat(pkgPath)
	assert.NoError(t, err)
}

// TODO: test DrainStderr
// TODO: test DrainStdouterr
// TODO: test UnreadStderr
// TODO: test UnreadStdouterr

func TestCmd_UnreadStdout(t *testing.T) {
	strs1 := []string{"testing123", "testing456"}
	strs2 := []string{"testing789", "testing012"}
	echoCmd := NewCmd("echo")
	echo := func(strs []string) {
		for _, str := range strs {
			{
				err := echoCmd.Run(str)
				if !assert.NoError(t, err) {
					t.Fatal(errs.Combine(errs.New("echo error"), err))
				}
			}

		}
	}
	check := func(strs []string) {
		stdout, err := echoCmd.UnreadStdout()
		if !assert.NoError(t, err) {
			t.Fatal(err)
		}
		for _, str := range strs {
			assert.Contains(t, stdout.String(), str)
		}
	}

	for _, strs := range [][]string{strs1, strs2} {
		echo(strs)
		check(strs)
	}
}

func TestCmd_DrainStdout(t *testing.T) {
	strs1 := []string{"testing123", "testing456"}
	strs2 := []string{"testing789", "testing012"}
	echoCmd := NewCmd("echo")
	echo := func(strs []string) {
		for _, str := range strs {
			{
				err := echoCmd.Run(str)
				if !assert.NoError(t, err) {
					t.Fatal(errs.Combine(errs.New("echo error"), err))
				}
			}

		}
	}

	echo(strs1)
	err := echoCmd.DrainStdout()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	echo(strs2)
	stdout, err := echoCmd.UnreadStdout()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	for _, str := range strs1 {
		assert.NotContains(t, stdout.String(), str)
	}
	for _, str := range strs2 {
		assert.Contains(t, stdout.String(), str)
	}
}
