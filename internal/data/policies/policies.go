package policies

import (
	"context"

	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"

	biz "github.com/project-kessel/inventory-api/internal/biz/policies"
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

func (r *policiesRepo) Save(context.Context, *biz.Policy) (*biz.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) Update(context.Context, *biz.Policy) (*biz.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *policiesRepo) FindByID(context.Context, int64) (*biz.Policy, error) {
	return nil, nil
}

func (r *policiesRepo) ListAll(context.Context) ([]*biz.Policy, error) {
	return nil, nil
}
