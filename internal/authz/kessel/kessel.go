package kessel

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"

	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
)

type KesselAuthz struct {
	CheckService kessel.KesselCheckServiceHTTPClient
	TupleService kessel.KesselTupleServiceHTTPClient

	Logger *log.Helper
}

var _ authzapi.Authorizer = &KesselAuthz{}

func New(ctx context.Context, config CompletedConfig, logger *log.Helper) (*KesselAuthz, error) {
	logger.Info("Using authorizer: kessel")
	return &KesselAuthz{
		CheckService: kessel.NewKesselCheckServiceHTTPClient(config.HttpClient),
		TupleService: kessel.NewKesselTupleServiceHTTPClient(config.HttpClient),

		Logger: logger,
	}, nil
}

func (a *KesselAuthz) Check(ctx context.Context, r *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	return a.CheckService.Check(ctx, r)
}

func (a *KesselAuthz) CreateTuples(ctx context.Context, r *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	return a.TupleService.CreateTuples(ctx, r)
}

func (a *KesselAuthz) DeleteTuples(ctx context.Context, r *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	return a.TupleService.DeleteTuples(ctx, r)
}
