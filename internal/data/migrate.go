package data

import (
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/project-kessel/inventory-api/internal/biz/resources/hosts"
	notifs "github.com/project-kessel/inventory-api/internal/biz/resources/notificationsintegrations"
	// "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	// "github.com/project-kessel/inventory-api/internal/biz/policies"
	k8spolicyrelations "github.com/project-kessel/inventory-api/internal/biz/relationships/k8spolicy"
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
		&common.RelationshipMetadata{},
		&common.RelationshipReporter{},
		// &k8sclusters.K8sCluster{},
		// &policies.Policy{},
		&k8spolicyrelations.K8SPolicyIsPropagatedToK8SCluster{},
	); err != nil {
		return err
	}
	logger.Info("Migration successful!")
	return nil
}
