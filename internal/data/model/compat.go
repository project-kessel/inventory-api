// Package model provides backward-compatible re-exports for GORM data model types
// that have been moved to internal/infrastructure/resourcerepository/gorm/.
// Remove these once all import sites have been updated.
package model

import (
	gormrepo "github.com/project-kessel/inventory-api/internal/infrastructure/resourcerepository/gorm"
)

// Type aliases.
type Resource = gormrepo.Resource
