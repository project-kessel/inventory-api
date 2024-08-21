package notificationsintegrations

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	ResourceType = "notifications/integration"
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

// CreateNotificationsIntegration creates a NotificationsIntegration in the repository and returns the new NotificationsIntegration.
func (uc *NotificationsIntegrationUsecase) CreateNotificationsIntegration(ctx context.Context, i *NotificationsIntegration) (*NotificationsIntegration, error) {
	if ret, err := uc.repo.Save(ctx, i); err != nil {
		return nil, err
	} else {
		uc.log.WithContext(ctx).Infof("CreateNotificationsIntegration: %v", i.ID)
		return ret, nil
	}
}
