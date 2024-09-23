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

// New creates a new a Host usecase.
func New(repo HostRepo, logger log.Logger) *HostUsecase {
	return &HostUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a Host in the repository and returns the new Host.
func (uc *HostUsecase) Create(ctx context.Context, h *Host) (*Host, error) {
	if ret, err := uc.repo.Save(ctx, h); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Create Host: %v", h.ID)
		return ret, nil
	}
}

// Update updates a Host in the repository and returns the updated Host.
func (uc *HostUsecase) Update(ctx context.Context, h *Host, id string) (*Host, error) {
	if ret, err := uc.repo.Update(ctx, h, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Update Host: %v", h.ID)
		return ret, nil
	}
}

// Delete deletes a Host in the repository.
func (uc *HostUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Delete Host: %v", id)
		return nil
	}
}
