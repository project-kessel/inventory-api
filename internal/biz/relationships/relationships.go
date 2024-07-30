package relationships

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type Relationship struct {
	Hello string
}

// RelationshipRepo is a Relationship repo.
type RelationshipRepo interface {
	Save(context.Context, *Relationship) (*Relationship, error)
	Update(context.Context, *Relationship) (*Relationship, error)
	FindByID(context.Context, int64) (*Relationship, error)
	ListAll(context.Context) ([]*Relationship, error)
}

// RelationshipUsecase is a Relationship usecase.
type RelationshipUsecase struct {
	repo RelationshipRepo
	log  *log.Helper
}

// New new a Relationship usecase.
func New(repo RelationshipRepo, logger log.Logger) *RelationshipUsecase {
	return &RelationshipUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateRelationship creates a Relationship, and returns the new Relationship.
func (uc *RelationshipUsecase) CreateRelationship(ctx context.Context, r *Relationship) (*Relationship, error) {
	uc.log.WithContext(ctx).Infof("CreateRelationship: %v", r.Hello)
	return uc.repo.Save(ctx, r)
}
