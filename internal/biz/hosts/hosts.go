package hosts

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "rhel-host"
)

// HostRepo is a Host repo.
type HostRepo interface {
	Save(context.Context, *Host) (*Host, error)
	Update(context.Context, *Host, string) (*Host, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*Host, error)
	ListAll(context.Context) ([]*Host, error)
}

// HostUsecase is a Host usecase.
type HostUsecase struct {
	repo HostRepo
	log  *log.Helper
}

// NewHostUsecase new a Host usecase.
func New(repo HostRepo, logger log.Logger) *HostUsecase {
	return &HostUsecase{repo: repo, log: log.NewHelper(logger)}
}

// CreateHost creates a Host in the repository and returns the new Host.
func (uc *HostUsecase) CreateHost(ctx context.Context, h *Host) (*Host, error) {
	if ret, err := uc.repo.Save(ctx, h); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("CreateHost: %v", h.ID)
		return ret, nil
	}
}
