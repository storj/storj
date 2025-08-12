// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/clingy"

	"storj.io/common/testcontext"
	"storj.io/storj/shared/modular"
	"storj.io/storj/shared/modular/config"
	"storj.io/storj/shared/mud"
)

type Task struct {
	done bool
}

func NewTask() *Task {
	return &Task{}
}

func (t *Task) Run() error {
	t.done = true
	return nil
}

type CustomSubcommandConfig struct {
	Activated bool `usage:"turn it on, to make it turned on"`
}

type CustomSubcommand struct {
	done bool
	cfg  *CustomSubcommandConfig
}

func NewCustomSubcommand(cfg *CustomSubcommandConfig) *CustomSubcommand {
	return &CustomSubcommand{
		cfg: cfg,
	}
}

func (c *CustomSubcommand) Run() error {
	c.done = true
	return nil
}

func TestRun(t *testing.T) {
	ctx := testcontext.New(t)

	t.Run("default run of Service annotated components", func(t *testing.T) {
		ball := mud.NewBall()
		mud.Provide[*Task](ball, NewTask)
		mud.Tag[*Task, modular.Service](ball, modular.Service{})
		mud.Provide[*ConfigList](ball, NewConfigList)

		cfg := &ConfigSupport{}
		ok, err := clingy.Environment{
			Dynamic: cfg.GetValue,
			Args:    []string{"exec"},
		}.Run(ctx, clingyRunner(cfg, ball))
		require.True(t, ok)
		require.NoError(t, err)
		c := mud.Find(ball, mud.Select[*Task](ball))
		require.Len(t, c, 1)
		require.NotNil(t, c[0].Instance())
		require.Equal(t, c[0].Instance().(*Task).done, true, "Task should be marked as done after execution")
	})

	t.Run("run configured components", func(t *testing.T) {
		ball := mud.NewBall()
		mud.Provide[*Task](ball, NewTask)
		mud.Provide[*ConfigList](ball, NewConfigList)

		cfg := &ConfigSupport{}
		ok, err := clingy.Environment{
			Dynamic: cfg.GetValue,
			Args:    []string{"exec", "--components=cli.Task"},
		}.Run(ctx, clingyRunner(cfg, ball))
		require.True(t, ok)
		require.NoError(t, err)
		c := mud.Find(ball, mud.Select[*Task](ball))
		require.Len(t, c, 1)
		require.NotNil(t, c[0].Instance())
		require.Equal(t, c[0].Instance().(*Task).done, true)
	})

	t.Run("run dynamic subcommand with config", func(t *testing.T) {
		ball := mud.NewBall()
		mud.Provide[*Task](ball, NewTask)
		mud.Provide[*ConfigList](ball, NewConfigList)
		mud.Provide[*CustomSubcommand](ball, NewCustomSubcommand)
		config.RegisterConfig[CustomSubcommandConfig](ball, "cs")
		RegisterSubcommand[*CustomSubcommand](ball, "custom", "Run a custom subcommand")

		cfg := &ConfigSupport{}
		ok, err := clingy.Environment{
			Dynamic: cfg.GetValue,
			Args:    []string{"custom", "-cs.activated=true"},
		}.Run(ctx, clingyRunner(cfg, ball))
		require.True(t, ok)
		require.NoError(t, err)

		c := mud.Find(ball, mud.Select[*Task](ball))
		require.Len(t, c, 1)
		require.Nil(t, c[0].Instance())

		c = mud.Find(ball, mud.Select[*CustomSubcommand](ball))
		require.Len(t, c, 1)
		require.NotNil(t, c[0].Instance())
		require.True(t, c[0].Instance().(*CustomSubcommand).done)
		require.True(t, c[0].Instance().(*CustomSubcommand).cfg.Activated)
	})
}
