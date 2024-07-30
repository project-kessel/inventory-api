package policies

import (
	"context"
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"

	models "github.com/project-kessel/inventory-api/internal/biz/policies"
)

type policiesRepo struct {
	DB  *gorm.DB
	Log *log.Helper
}

func New(g *gorm.DB, l *log.Helper) *policiesRepo {
	return &policiesRepo{
		DB:  g,
		Log: l,
	}
}

func (r *policiesRepo) Save(context.Context, *models.Policy) (*models.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) Update(context.Context, *models.Policy) (*models.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *policiesRepo) FindByID(context.Context, int64) (*models.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) ListAll(context.Context) ([]*models.Policy, error) {
	return nil, nil
}
