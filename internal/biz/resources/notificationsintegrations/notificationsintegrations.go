package notificationsintegrations

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "notifications-integration"
)

// NotificationsIntegration is a NotificationsIntegration repo.
type NotificationsIntegrationRepo interface {
	Save(context.Context, *NotificationsIntegration) (*NotificationsIntegration, error)
	Update(context.Context, *NotificationsIntegration, string) (*NotificationsIntegration, error)
	Delete(context.Context, string) error
	FindByID(context.Context, string) (*NotificationsIntegration, error)
	ListAll(context.Context) ([]*NotificationsIntegration, error)
}

// NotificationsUsecase is a Notifications usecase.
type NotificationsIntegrationUsecase struct {
	repo NotificationsIntegrationRepo
	log  *log.Helper
}

// NewNotificationsIntegrationUsecase new a NotificationsIntegration usecase.
func New(repo NotificationsIntegrationRepo, logger log.Logger) *NotificationsIntegrationUsecase {
	return &NotificationsIntegrationUsecase{repo: repo, log: log.NewHelper(logger)}
}

// Create creates a NotificationsIntegration in the repository and returns the new NotificationsIntegration.
func (uc *NotificationsIntegrationUsecase) Create(ctx context.Context, i *NotificationsIntegration) (*NotificationsIntegration, error) {
	if ret, err := uc.repo.Save(ctx, i); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Create Notifications Integration: %v", i.ID)
		return ret, nil
	}
}

// Update updates a NotificationsIntegration in the repository and returns the updated NotificationsIntegration.
func (uc *NotificationsIntegrationUsecase) Update(ctx context.Context, i *NotificationsIntegration, id string) (*NotificationsIntegration, error) {
	if ret, err := uc.repo.Update(ctx, i, id); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("Update Notifications Integration: %v", i.ID)
		return ret, nil
	}
}

// Delete deletes a NotificationsIntegration in the repository.
func (uc *NotificationsIntegrationUsecase) Delete(ctx context.Context, id string) error {
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	} else {
		uc.log.WithContext(ctx).Infof("Delete Notifications Integration: %v", id)
		return nil
	}
}
