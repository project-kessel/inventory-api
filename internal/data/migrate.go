package data

import (
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/project-kessel/inventory-api/internal/biz/hosts"
	notifs "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	// "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	// "github.com/project-kessel/inventory-api/internal/biz/policies"
	// "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// Migrate the tables
// See https://gorm.io/docs/migration.html
func Migrate(db *gorm.DB, logger *log.Helper) error {
	if err := db.AutoMigrate(
		&notifs.NotificationsIntegration{},
		&hosts.Host{},
		&common.Metadata{},
		&common.Reporter{},
		&common.Label{},
		// &k8sclusters.K8sCluster{},
		// &policies.Policy{},
		// &relationships.Relationship{},
	); err != nil {
		return err
	}
	logger.Info("Migration successful!")
	return nil
}
