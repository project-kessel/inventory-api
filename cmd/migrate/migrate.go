package migrate

import (
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/spf13/cobra"

	"github.com/project-kessel/inventory-api/internal/errors"
	gormrepo "github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/gorm"
	"github.com/project-kessel/inventory-api/internal/provider"
)

// NewCommand creates a new cobra command for database migration.
// It creates or migrates the database tables using the provided storage options.
func NewCommand(options *provider.StorageOptions, loggerOptions common.LoggerOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Create or migrate the database tables",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
			logHelper := log.NewHelper(log.With(logger, "group", "storage"))

			if errs := options.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}

			db, err := provider.NewStorage(options, logHelper)
			if err != nil {
				return err
			}

			return gormrepo.Migrate(db, logHelper)
		},
	}

	return cmd
}
