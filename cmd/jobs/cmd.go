package jobs

import (
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/storage"
	"github.com/spf13/cobra"
)

// To add a job, add a file in this directory named after the job
// Then create a cobra.Command and function that it will run
// Finally, add the command to the run-job cobra.Command
func NewRunJobCommand(storageOptions *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run-job",
		Short: "Run an inventory service job",
		Long:  "Run an inventory service job",
	}

	cmds := []*cobra.Command{
		NewResourceDeleteJobCommand(storageOptions, loggerOptions),
	}

	for _, c := range cmds {
		cmd.AddCommand(c)
	}

	return cmd
}
