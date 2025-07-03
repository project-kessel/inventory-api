package health_test

import (
	"context"
	"testing"

	"github.com/spf13/viper"

	"github.com/stretchr/testify/assert"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/mocks"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	biz "github.com/project-kessel/inventory-api/internal/biz/health"
	svc "github.com/project-kessel/inventory-api/internal/service/health"
)

func TestGetLivez(t *testing.T) {
	ctx := context.TODO()
	s := svc.New(nil)

	viper.Set("log.livez", true)
	resp, err := s.GetLivez(ctx, &pb.GetLivezRequest{})
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), resp.Code)
	assert.Equal(t, "OK", resp.Status)

	viper.Set("log.livez", false)
	resp, err = s.GetLivez(ctx, &pb.GetLivezRequest{})
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), resp.Code)
}

func TestHealthService_GetReadyz(t *testing.T) {
	ctx := context.TODO()

	mockRepo := new(mocks.MockHealthRepo)
	expectedResp := &pb.GetReadyzResponse{Status: "MOCK_OK", Code: 200}
	mockRepo.On("IsBackendAvailable", ctx).Return(expectedResp, nil)

	uc := biz.New(mockRepo, log.DefaultLogger, false)
	service := svc.New(uc)

	resp, err := service.GetReadyz(ctx, &pb.GetReadyzRequest{})
	assert.NoError(t, err)
	assert.Equal(t, uint32(200), resp.Code)
	assert.Equal(t, "MOCK_OK", resp.Status)

	mockRepo.AssertExpectations(t)
}
