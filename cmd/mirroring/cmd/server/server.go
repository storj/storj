package server

import (
	"github.com/spf13/cobra"
	minio "github.com/minio/minio/cmd"
)

var Cmd = &cobra.Command{
	Use: "server",

	Args:  nil,
	Short: "Upload files or file list to specified bucket",
	PreRunE: preRunE,
	Long: `Upload files or file list to specified bucket`,
	Run:  run,
}

func preRunE(cmd *cobra.Command, args []string) error {
	return nil
}

func run(cmd *cobra.Command, args []string) {
	minio.Main([]string{"mirroring", "gateway", "mirroring"})
}

func init() {

}