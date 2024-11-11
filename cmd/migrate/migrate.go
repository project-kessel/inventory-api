package migrate

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/spf13/cobra"

	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/errors"
	"github.com/project-kessel/inventory-api/internal/storage"
)

func NewCommand(options *storage.Options, loggerOptions common.LoggerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Create or migrate the database tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
			logHelper := log.NewHelper(log.With(logger, "group", "storage"))

			if options.DisablePersistence {
				logHelper.Info("Persistence disabled, skipping database migration...")
				return nil
			}

			if errs := options.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}

			if errs := options.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}

			config := storage.NewConfig(options).Complete()

			db, err := storage.New(config, logHelper)
			if err != nil {
				return err
			}

			return data.Migrate(db, logHelper)
		},
	}

	return cmd
}
