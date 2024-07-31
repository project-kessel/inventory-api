package data

import (
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/internal/biz/common"
	"github.com/project-kessel/inventory-api/internal/biz/hosts"
	// "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	// "github.com/project-kessel/inventory-api/internal/biz/policies"
	// "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

// Migrate the tables
// See https://gorm.io/docs/migration.html
func Migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&hosts.Host{},
		&common.Metadata{},
		&common.Reporter{},
		&common.Tag{},
		// &k8sclusters.K8sCluster{},
		// &policies.Policy{},
		// &relationships.Relationship{},
	); err != nil {
		return err
	}
	return nil
}
