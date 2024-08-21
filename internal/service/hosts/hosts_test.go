package hosts

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"
	"github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/hosts"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

type HostRepoMock struct {
	mock.Mock
}

func (m *HostRepoMock) Save(ctx context.Context, host *hosts.Host) (*hosts.Host, error) {
	return host, nil
}

func (m *HostRepoMock) Update(ctx context.Context, host *hosts.Host, hostId string) (*hosts.Host, error) {
	return host, nil
}

func (m *HostRepoMock) Delete(ctx context.Context, hostId string) error {
	return nil
}

func (m *HostRepoMock) FindByID(ctx context.Context, hostId string) (*hosts.Host, error) {
	return nil, nil
}

func (m *HostRepoMock) ListAll(ctx context.Context) ([]*hosts.Host, error) {
	return []*hosts.Host{}, nil
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

func TestCreateHostWithRequiredDataIsSuccess(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}

	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		RhelHost: &pb.RhelHost{
			Metadata: nil,
			ReporterData: &pb.ReporterData{
				ReporterType:    pb.ReporterData_REPORTER_TYPE_OCM,
				LocalResourceId: "testing",
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.NoError(t, err)
}

func TestCreateHostWithOptionalAttributesIsSuccess(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}

	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		RhelHost: &pb.RhelHost{
			Metadata: &pb.Metadata{
				Labels:    []*pb.ResourceLabel{},
				Workspace: "foobar",
			},
			ReporterData: &pb.ReporterData{
				ReporterType:    pb.ReporterData_REPORTER_TYPE_OCM,
				LocalResourceId: "testing",
				ConsoleHref:     "http://my-console",
				ApiHref:         "http://my-api",
				ReporterVersion: "1337",
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.NoError(t, err)
}

func TestCreateInvalidHostIsBadRequest(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}
	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		RhelHost: &pb.RhelHost{
			Metadata: nil,
			ReporterData: &pb.ReporterData{
				ReporterType: pb.ReporterData_REPORTER_TYPE_OCM,
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.ErrorContains(t, err, "400")
}
