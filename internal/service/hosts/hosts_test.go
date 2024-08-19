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
	"strconv"
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
		Host: &pb.RhelHost{
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

func TestCreateHostWithLabelsIsSuccess(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}

	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		Host: &pb.RhelHost{
			Metadata: &pb.Metadata{
				Labels: []*pb.ResourceLabel{
					{
						Key:   "foo",
						Value: "bar",
					},
					{
						Key:   "foo",
						Value: "baz",
					},
				},
			},
			ReporterData: &pb.ReporterData{
				ReporterType:    pb.ReporterData_REPORTER_TYPE_OCM,
				LocalResourceId: "testing",
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.NoError(t, err)
}

func TestCreateHostWithEmptyLabelArrayIsSuccess(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}

	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		Host: &pb.RhelHost{
			Metadata: &pb.Metadata{
				Labels: []*pb.ResourceLabel{},
			},
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
		Host: &pb.RhelHost{
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

func TestCreateHostWithInvalidReporterTypesBadRequest(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}
	ctx := mockContext()

	invalidReporterTypes := []int32{
		int32(pb.ReporterData_REPORTER_TYPE_UNSPECIFIED),
		15,
		99,
	}

	for _, reporterType := range invalidReporterTypes {
		t.Run(strconv.Itoa(int(reporterType)), func(t *testing.T) {
			request := pb.CreateRhelHostRequest{
				Host: &pb.RhelHost{
					Metadata: nil,
					ReporterData: &pb.ReporterData{
						LocalResourceId: "testing",
						ReporterType:    pb.ReporterData_ReporterType(reporterType),
					},
				},
			}

			_, err := service.CreateRhelHost(ctx, &request)

			assert.ErrorContains(t, err, "400")
			assert.ErrorContains(t, err, "invalid RhelHost.ReporterData")
			assert.ErrorContains(t, err, "invalid ReporterData.ReporterType")
		})
	}
}

func TestCreateHostWithInvalidLocalResourceIdBadRequest(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}
	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		Host: &pb.RhelHost{
			Metadata: nil,
			ReporterData: &pb.ReporterData{
				ReporterType: pb.ReporterData_REPORTER_TYPE_OCM,
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.ErrorContains(t, err, "400")
	assert.ErrorContains(t, err, "invalid RhelHost.ReporterData")
	assert.ErrorContains(t, err, "invalid ReporterData.LocalResourceId")
}

func TestCreateHostWithInvalidLabelIsBadRequest(t *testing.T) {
	repo := new(HostRepoMock)
	hostUsecase := hosts.New(repo, log.DefaultLogger)

	service := HostsService{
		Ctl: hostUsecase,
	}
	ctx := mockContext()

	request := pb.CreateRhelHostRequest{
		Host: &pb.RhelHost{
			Metadata: &pb.Metadata{
				Labels: []*pb.ResourceLabel{
					{
						Key: "",
					},
				},
			},
			ReporterData: &pb.ReporterData{
				ReporterType:    pb.ReporterData_REPORTER_TYPE_OCM,
				LocalResourceId: "resource-id",
			},
		},
	}

	_, err := service.CreateRhelHost(ctx, &request)

	assert.ErrorContains(t, err, "400")
	assert.ErrorContains(t, err, "invalid RhelHost.Metadata")
	assert.ErrorContains(t, err, "invalid Metadata.Labels")
}