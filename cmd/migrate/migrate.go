package migrate

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/spf13/cobra"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/errors"
	"github.com/project-kessel/inventory-api/internal/storage"
)

func NewCommand(options *storage.Options, logger log.Logger) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Create or migrate the database tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := options.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}

			if errs := options.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}

			config := storage.NewConfig(options).Complete()

			logHelper := log.NewHelper(log.With(logger, "group", "storage"))
			db, err := storage.New(config, logHelper)
			if err != nil {
				return err
			}

			return data.Migrate(db, logHelper)
		},
	}

	return cmd
}
