package put

import (
	"storj.io/mirroring/utils"
	gw "storj.io/mirroring/pkg/gateway"
	"os/signal"
	"os"
	"github.com/spf13/cobra"
)

// Cmd represents the put command
var Cmd = &cobra.Command{
	Use: "put [bucket name] [path to file/folder]",
	Args:    validateArgs,
	Short:   "Upload files or file list to specified bucket",
	Long:    `Upload files or file list to specified bucket`,
	RunE:    NewPutExec(&gw.Mirroring{Logger:&utils.StdOutLogger}, &utils.StdOutLogger).runE,
}

var (
	frecursive, fforce bool
	fprefix, fdelimiter string // TODO: implement delimiter flag

	sigc chan os.Signal
)

func init() {
	sigc = make(chan os.Signal)
	signal.Notify(sigc, os.Interrupt)

	Cmd.Flags().BoolVarP(&frecursive, "recursive", "r", false, "recursive usage")
	Cmd.Flags().BoolVarP(&fforce, "force", "f", false, "force usage")
	Cmd.Flags().StringVarP(&fprefix, "prefix", "p", "", "prefix usage")
	Cmd.Flags().StringVarP(&fdelimiter, "delimiter", "d", "/", "delimiter usage")
}