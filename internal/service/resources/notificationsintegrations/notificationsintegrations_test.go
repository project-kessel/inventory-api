package notificationsintegrations

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	biz "github.com/project-kessel/inventory-api/internal/biz/resources/notificationsintegrations"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type NotificationIntegrationsRepoMock struct {
	mock.Mock
}

func (m *NotificationIntegrationsRepoMock) Save(ctx context.Context, integration *biz.NotificationsIntegration) (*biz.NotificationsIntegration, error) {
	return integration, nil
}

func (m *NotificationIntegrationsRepoMock) Update(ctx context.Context, integration *biz.NotificationsIntegration, id string) (*biz.NotificationsIntegration, error) {
	return integration, nil
}

func (m *NotificationIntegrationsRepoMock) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *NotificationIntegrationsRepoMock) FindByID(ctx context.Context, id string) (*biz.NotificationsIntegration, error) {
	return nil, nil
}

func (m *NotificationIntegrationsRepoMock) ListAll(ctx context.Context) ([]*biz.NotificationsIntegration, error) {
	return []*biz.NotificationsIntegration{}, nil
}

func mockContext() context.Context {
	return context.WithValue(
		context.Background(),
		middleware.IdentityRequestKey,
		&api.Identity{
			Principal: "test-principal",
		},
	)
}

func TestCreateNotificationIntegrationWithRequiredDataIsSuccess(t *testing.T) {
	repo := new(NotificationIntegrationsRepoMock)
	notificationsintegrationsUsecase := biz.New(repo, log.DefaultLogger)

	service := NotificationsIntegrationsService{
		Ctl: notificationsintegrationsUsecase,
	}

	ctx := mockContext()

	request := pb.CreateNotificationsIntegrationRequest{
		Integration: &pb.NotificationsIntegration{
			Metadata: nil,
			ReporterData: &pb.ReporterData{
				ReporterType:    pb.ReporterData_HBI,
				LocalResourceId: "testing",
			},
		},
	}

	_, err := service.CreateNotificationsIntegration(ctx, &request)

	assert.NoError(t, err)
}
