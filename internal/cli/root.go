package cli

import (
	"github.com/spf13/cobra"
)

var (
	serverAddr string
	configFile string
	verbose    bool
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "seedee",
		Short: "seedee — a CI system",
		Long: `seedee is a CI system that runs pipelines defined in .seedee.yml.

Pipelines execute jobs in parallel inside Docker containers.
Run locally (default) or against a remote seedee server.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&serverAddr, "server", "", "remote seedee server address (e.g., localhost:8080)")
	root.PersistentFlags().StringVarP(&configFile, "config", "c", ".seedee.yml", "path to pipeline config file")
	root.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")

	root.AddCommand(
		newRunCmd(),
		newStatusCmd(),
		newCancelCmd(),
		newListCmd(),
		newVersionCmd(),
	)

	return root
}
