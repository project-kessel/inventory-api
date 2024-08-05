package relationships

import (
	"context"

	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"

	biz "github.com/project-kessel/inventory-api/internal/biz/relationships"
)

type relationshipsRepo struct {
	DB  *gorm.DB
	Log *log.Helper
}

func New(g *gorm.DB, l *log.Helper) *relationshipsRepo {
	return &relationshipsRepo{
		DB:  g,
		Log: l,
	}
}

func (r *relationshipsRepo) Save(context.Context, *biz.Relationship) (*biz.Relationship, error) {
	return nil, nil
}

func (r *relationshipsRepo) Update(context.Context, *biz.Relationship) (*biz.Relationship, error) {
	return nil, nil
}

func (r *relationshipsRepo) Delete(context.Context, int64) error {
	return nil
}

func (r *relationshipsRepo) FindByID(context.Context, int64) (*biz.Relationship, error) {
	return nil, nil
}

func (r *relationshipsRepo) ListAll(context.Context) ([]*biz.Relationship, error) {
	return nil, nil
}
