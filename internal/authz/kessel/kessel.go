package kessel

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/spf13/viper"

	"github.com/go-kratos/kratos/v2/log"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kessel "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

type KesselAuthz struct {
	HealthService  kesselv1.KesselRelationsHealthServiceClient
	CheckService   kessel.KesselCheckServiceClient
	TupleService   kessel.KesselTupleServiceClient
	LookupService  kessel.KesselLookupServiceClient
	tokenClient    *tokenClient
	Logger         *log.Helper
	successCounter metric.Int64Counter
	failureCounter metric.Int64Counter
}

var _ authzapi.Authorizer = &KesselAuthz{}

func New(ctx context.Context, config CompletedConfig, logger *log.Helper) (*KesselAuthz, error) {
	logger.Info("Using authorizer: kessel")
	tokenCli := NewTokenClient(config.tokenConfig)

	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")

	successCounter, err := meter.Int64Counter("inventory_relations_api_success")
	if err != nil {
		return nil, fmt.Errorf("failed to create success counter: %w", err)
	}

	failureCounter, err := meter.Int64Counter("inventory_relations_api_failure")
	if err != nil {
		return nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	return &KesselAuthz{
		HealthService:  kesselv1.NewKesselRelationsHealthServiceClient(config.gRPCConn),
		CheckService:   kessel.NewKesselCheckServiceClient(config.gRPCConn),
		TupleService:   kessel.NewKesselTupleServiceClient(config.gRPCConn),
		Logger:         logger,
		tokenClient:    tokenCli,
		successCounter: successCounter,
		failureCounter: failureCounter,
	}, nil
}

func (a *KesselAuthz) incrFailureCounter(method string) {
	a.failureCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (a *KesselAuthz) incrSuccessCounter(method string) {
	a.successCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (a *KesselAuthz) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("Health")
		return nil, err
	}
	if viper.GetBool("log.readyz") {
		log.Infof("Checking relations-api readyz endpoint")
	}
	resp, err := a.HealthService.GetReadyz(ctx, &kesselv1.GetReadyzRequest{}, opts...)
	if err != nil {
		a.incrFailureCounter("Health")
		return nil, err
	}

	a.incrSuccessCounter("Health")
	return resp, nil
}

func (a *KesselAuthz) getCallOptions() ([]grpc.CallOption, error) {
	var opts []grpc.CallOption
	opts = append(opts, grpc.EmptyCallOption{})
	if a.tokenClient.EnableOIDCAuth {
		token, err := a.tokenClient.getToken()
		if err != nil {
			return nil, fmt.Errorf("failed to request token: %w", err)
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
		a.incrFailureCounter("CreateTuples")
		return nil, err
	}

	resp, err := a.TupleService.CreateTuples(ctx, r, opts...)
	if err != nil {
		a.incrFailureCounter("CreateTuples")
		return nil, err
	}

	a.incrSuccessCounter("CreateTuples")
	return resp, nil
}

func (a *KesselAuthz) DeleteTuples(ctx context.Context, r *kessel.DeleteTuplesRequest) (*kessel.DeleteTuplesResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return nil, err
	}

	resp, err := a.TupleService.DeleteTuples(ctx, r, opts...)
	if err != nil {
		a.incrFailureCounter("DeleteTuples")
		return nil, err
	}

	a.incrSuccessCounter("DeleteTuples")
	return resp, nil
}

func (a *KesselAuthz) UnsetWorkspace(ctx context.Context, local_resource_id, namespace, name string) (*kessel.DeleteTuplesResponse, error) {

	req := &kessel.RelationTupleFilter{
		ResourceNamespace: proto.String(namespace),
		ResourceType:      proto.String(name),
		ResourceId:        proto.String(local_resource_id),
		Relation:          proto.String("workspace"),
	}
	return a.DeleteTuples(ctx, &kessel.DeleteTuplesRequest{
		Filter: req,
	})
}

func (a *KesselAuthz) Check(ctx context.Context, namespace string, viewPermission string, resource *model.Resource, sub *kessel.SubjectReference) (kessel.CheckResponse_Allowed, *kessel.ConsistencyToken, error) {
	log.Infof("Check: on %+v", resource)
	consistency := &kessel.Consistency{Requirement: &kessel.Consistency_MinimizeLatency{MinimizeLatency: true}}

	if resource.ConsistencyToken != "" {
		consistency = &kessel.Consistency{
			Requirement: &kessel.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kessel.ConsistencyToken{Token: resource.ConsistencyToken},
			},
		}
	}

	resp, err := a.CheckService.Check(ctx, &kessel.CheckRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: namespace,
				Name:      resource.ResourceType,
			},
			Id: resource.ReporterResourceId,
		},
		Relation:    viewPermission,
		Subject:     sub,
		Consistency: consistency,
	})

	log.Infof("CheckForView resp: %v err: %v", resp, err)

	if err != nil {
		return kessel.CheckResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	return resp.GetAllowed(), resp.GetConsistencyToken(), nil
}

func (a *KesselAuthz) CheckForUpdate(ctx context.Context, namespace string, updatePermission string, resource *model.Resource, sub *kessel.SubjectReference) (kessel.CheckForUpdateResponse_Allowed, *kessel.ConsistencyToken, error) {
	resp, err := a.CheckService.CheckForUpdate(ctx, &kessel.CheckForUpdateRequest{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Namespace: namespace,
				Name:      resource.ResourceType,
			},
			Id: resource.ReporterResourceId,
		},
		Relation: updatePermission,
		Subject:  sub,
	})

	if err != nil {
		return kessel.CheckForUpdateResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	return resp.GetAllowed(), resp.GetConsistencyToken(), nil
}

func (a *KesselAuthz) SetWorkspace(ctx context.Context, local_resource_id, workspace, namespace, name string) (*kessel.CreateTuplesResponse, error) {
	if workspace == "" {
		a.incrFailureCounter("SetWorkspace")
		return nil, fmt.Errorf("workspace_id is required")
	}
	// TODO: remove previous tuple for workspace
	rels := []*kessel.Relationship{{
		Resource: &kessel.ObjectReference{
			Type: &kessel.ObjectType{
				Name:      name,
				Namespace: namespace,
			},
			Id: local_resource_id,
		},
		Relation: "workspace",
		Subject: &kessel.SubjectReference{
			Subject: &kessel.ObjectReference{
				Type: &kessel.ObjectType{
					Name:      "workspace",
					Namespace: "rbac",
				},
				Id: workspace,
			},
		},
	}}

	a.incrSuccessCounter("SetWorkspace")
	return a.CreateTuples(ctx, &kessel.CreateTuplesRequest{
		Tuples: rels,
	})
}
