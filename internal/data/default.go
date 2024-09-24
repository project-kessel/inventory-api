package data

import (
	"context"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"gorm.io/gorm"
)

type defaultRepository struct {
	DB      *gorm.DB
	Authz   authzapi.Authorizer
	Eventer eventingapi.Manager
}

func New(g *gorm.DB, a authzapi.Authorizer, e eventingapi.Manager) *defaultRepository {
	return &defaultRepository{
		DB:      g,
		Authz:   a,
		Eventer: e,
	}
}

func (dr *defaultRepository) Save(ctx context.Context, model *interface{}) (*interface{}, error) {
	return nil, nil
}
