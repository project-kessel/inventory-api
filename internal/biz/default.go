package biz

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
)

type Keys interface {
	~string | ~int64
}

type DefaultRepository[T any, K Keys] interface {
	Save(context.Context, *T) (*T, error)
	Update(context.Context, *T, K) (*T, error)
	Delete(context.Context, K) error
	FindByID(context.Context, K) (*T, error)
	ListAll(context.Context) ([]*T, error)
}

type DefaultUsecase[T any, K Keys] struct {
	repo DefaultRepository[T, K]
	log  *log.Helper
}

func New[T any, K Keys](repo DefaultRepository[T, K], logger log.Logger) *DefaultUsecase[T, K] {
	return &DefaultUsecase[T, K]{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

func (uc *DefaultUsecase[T, K]) Create(ctx context.Context, t *T) (*T, error) {
	if ret, err := uc.repo.Save(ctx, t); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Created with DefaultUsecase: %v", t)
		return ret, nil
	}
}

func (uc *DefaultUsecase[T, K]) Update(ctx context.Context, t *T, id K) (*T, error) {
	if ret, err := uc.repo.Update(ctx, t, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Updated with DefaultUsecase: %v", t)
		return ret, nil
	}
}

func (uc *DefaultUsecase[T, K]) Delete(ctx context.Context, id K) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Deleted by ID with DefaultUsecase: %v", id)
		return nil
	}
}
