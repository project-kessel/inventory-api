package kessel

import (
	"context"
	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"google.golang.org/grpc"
)

type KesselAuthz struct {
	CheckService kessel.KesselCheckServiceClient
	TupleService kessel.KesselTupleServiceClient
	tokenClient  *tokenClient
	Logger       *log.Helper
}

var _ authzapi.Authorizer = &KesselAuthz{}

func New(ctx context.Context, config CompletedConfig, logger *log.Helper) (*KesselAuthz, error) {
	logger.Info("Using authorizer: kessel")
	tokenCli := NewTokenClient(config.tokenConfig)

	return &KesselAuthz{
		CheckService: kessel.NewKesselCheckServiceClient(config.gRPCConn),
		TupleService: kessel.NewKesselTupleServiceClient(config.gRPCConn),
		Logger:       logger,
		tokenClient:  tokenCli,
	}, nil
}

func (a *KesselAuthz) Check(ctx context.Context, r *kessel.CheckRequest) (*kessel.CheckResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		return nil, err
	}
	return a.CheckService.Check(ctx, r, opts...)
}

func (a *KesselAuthz) getCallOptions() ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	opts = append(opts, grpc.EmptyCallOption{})
	if a.tokenClient.EnableOIDCAuth {
		token, err := a.tokenClient.getToken()
		if err != nil {
			return nil, err
		}
		if a.tokenClient.Insecure {
			opts = append(opts, WithInsecureBearerToken(token.AccessToken))
		} else {
			opts = append(opts, WithBearerToken(token.AccessToken))
		}
	}
	return opts, nil
}

func (a *KesselAuthz) CreateTuples(ctx context.Context, r *kessel.CreateTuplesRequest) (*kessel.CreateTuplesResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		return nil, err
	}
	return a.TupleService.CreateTuples(ctx, r, opts...)
}

func (a *KesselAuthz) DeleteTuples(ctx context.Context, r *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		return nil, err
	}
	return a.TupleService.DeleteTuples(ctx, r, opts...)
}

func (a *KesselAuthz) KesselStatus(ctx context.Context) bool {
	return true
}
