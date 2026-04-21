package data

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/spf13/viper"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/config/relations/kessel"
	kesselv1 "github.com/project-kessel/relations-api/api/kessel/relations/v1"
	kesselapi "github.com/project-kessel/relations-api/api/kessel/relations/v1beta1"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
)

type GRPCRelationsRepository struct {
	HealthService  kesselv1.KesselRelationsHealthServiceClient
	CheckService   kesselapi.KesselCheckServiceClient
	TupleService   kesselapi.KesselTupleServiceClient
	LookupService  kesselapi.KesselLookupServiceClient
	tokenClient    *tokenClient
	Logger         *log.Helper
	successCounter metric.Int64Counter
	failureCounter metric.Int64Counter
}

var _ model.RelationsRepository = &GRPCRelationsRepository{}

func NewGRPCRelationsRepository(ctx context.Context, config kessel.CompletedConfig, logger *log.Helper) (*GRPCRelationsRepository, error) {
	logger.Info("Using relations repository: kessel")
	tokenCli := NewTokenClient(config.GetTokenConfig())

	meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")

	successCounter, err := meter.Int64Counter("inventory_relations_api_success")
	if err != nil {
		return nil, fmt.Errorf("failed to create success counter: %w", err)
	}

	failureCounter, err := meter.Int64Counter("inventory_relations_api_failure")
	if err != nil {
		return nil, fmt.Errorf("failed to create failure counter: %w", err)
	}

	return &GRPCRelationsRepository{
		HealthService:  kesselv1.NewKesselRelationsHealthServiceClient(config.GetGRPCConn()),
		CheckService:   kesselapi.NewKesselCheckServiceClient(config.GetGRPCConn()),
		TupleService:   kesselapi.NewKesselTupleServiceClient(config.GetGRPCConn()),
		LookupService:  kesselapi.NewKesselLookupServiceClient(config.GetGRPCConn()),
		Logger:         logger,
		tokenClient:    tokenCli,
		successCounter: successCounter,
		failureCounter: failureCounter,
	}, nil
}

func (a *GRPCRelationsRepository) incrFailureCounter(method string) {
	a.failureCounter.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("method", method),
	))
}

func (a *GRPCRelationsRepository) incrSuccessCounter(method string) {
	a.successCounter.Add(context.Background(), 1, metric.WithAttributes(attribute.String("method", method)))
}

func (a *GRPCRelationsRepository) Health(ctx context.Context) (*kesselv1.GetReadyzResponse, error) {
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

func (a *GRPCRelationsRepository) getCallOptions() ([]grpc.CallOption, error) {
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

func (a *GRPCRelationsRepository) AcquireLock(ctx context.Context, r *kesselapi.AcquireLockRequest) (*kesselapi.AcquireLockResponse, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return nil, err
	}

	resp, err := a.TupleService.AcquireLock(ctx, r, opts...)
	if err != nil {
		a.incrFailureCounter("AcquireLock")
		return nil, err
	}

	a.incrSuccessCounter("AcquireLock")
	return resp, nil
}

func (a *GRPCRelationsRepository) CreateTuples(ctx context.Context, r *kesselapi.CreateTuplesRequest) (*kesselapi.CreateTuplesResponse, error) {
	log.Infof("Creating tuples : %s", r)
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

func (a *GRPCRelationsRepository) DeleteTuples(ctx context.Context, r *kesselapi.DeleteTuplesRequest) (*kesselapi.DeleteTuplesResponse, error) {
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

func (a *GRPCRelationsRepository) LookupResources(ctx context.Context, in *kesselapi.LookupResourcesRequest) (grpc.ServerStreamingClient[kesselapi.LookupResourcesResponse], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupResources")
		return nil, err
	}
	resp, err := a.LookupService.LookupResources(ctx, in, opts...)
	if err != nil {
		a.incrFailureCounter("LookupResources")
		return nil, err
	}
	return resp, nil
}

func (a *GRPCRelationsRepository) LookupSubjects(ctx context.Context, in *kesselapi.LookupSubjectsRequest) (grpc.ServerStreamingClient[kesselapi.LookupSubjectsResponse], error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, err
	}
	resp, err := a.LookupService.LookupSubjects(ctx, in, opts...)
	if err != nil {
		a.incrFailureCounter("LookupSubjects")
		return nil, err
	}
	return resp, nil
}

func (a *GRPCRelationsRepository) UnsetWorkspace(ctx context.Context, local_resource_id, namespace, name string) (*kesselapi.DeleteTuplesResponse, error) {

	req := &kesselapi.RelationTupleFilter{
		ResourceNamespace: proto.String(namespace),
		ResourceType:      proto.String(name),
		ResourceId:        proto.String(local_resource_id),
		Relation:          proto.String("workspace"),
	}
	return a.DeleteTuples(ctx, &kesselapi.DeleteTuplesRequest{
		Filter: req,
	})
}

func (a *GRPCRelationsRepository) Check(ctx context.Context, namespace string, viewPermission string, consistencyToken string, resourceType string, localResourceId string, sub *kesselapi.SubjectReference) (kesselapi.CheckResponse_Allowed, *kesselapi.ConsistencyToken, error) {
	log.Infof("Check: on resourceType=%s, localResourceId=%s, consistencyToken=%s", resourceType, localResourceId, consistencyToken)

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("Check")
		return kesselapi.CheckResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	// If resource doesn't exist in inventory DB
	// default send a minimize_latency check request
	consistency := &kesselapi.Consistency{Requirement: &kesselapi.Consistency_MinimizeLatency{MinimizeLatency: true}}

	if consistencyToken != "" {
		log.Infof("Check: with Consistency_AtLeastAsFresh as consistencyToken=%s", consistencyToken)
		consistency = &kesselapi.Consistency{
			Requirement: &kesselapi.Consistency_AtLeastAsFresh{
				AtLeastAsFresh: &kesselapi.ConsistencyToken{Token: consistencyToken},
			},
		}
	}

	resp, err := a.CheckService.Check(ctx, &kesselapi.CheckRequest{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: namespace,
				Name:      resourceType,
			},
			Id: localResourceId,
		},
		Relation:    viewPermission,
		Subject:     sub,
		Consistency: consistency,
	}, opts...)

	log.Infof("CheckForView resp: %v err: %v", resp, err)

	if err != nil {
		a.incrFailureCounter("Check")
		return kesselapi.CheckResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	a.incrSuccessCounter("Check")
	return resp.GetAllowed(), resp.GetConsistencyToken(), nil
}

func (a *GRPCRelationsRepository) CheckForUpdate(ctx context.Context, namespace string, updatePermission string, resourceType string, localResourceId string, sub *kesselapi.SubjectReference) (kesselapi.CheckForUpdateResponse_Allowed, *kesselapi.ConsistencyToken, error) {
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return kesselapi.CheckForUpdateResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	resp, err := a.CheckService.CheckForUpdate(ctx, &kesselapi.CheckForUpdateRequest{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Namespace: namespace,
				Name:      resourceType,
			},
			Id: localResourceId,
		},
		Relation: updatePermission,
		Subject:  sub,
	}, opts...)

	if err != nil {
		a.incrFailureCounter("CheckForUpdate")
		return kesselapi.CheckForUpdateResponse_ALLOWED_UNSPECIFIED, nil, err
	}

	a.incrSuccessCounter("CheckForUpdate")
	return resp.GetAllowed(), resp.GetConsistencyToken(), nil
}

func (a *GRPCRelationsRepository) CheckBulk(ctx context.Context, req *kesselapi.CheckBulkRequest) (*kesselapi.CheckBulkResponse, error) {

	log.Infof("CheckBulk: checking %d items", len(req.GetItems()))

	if req.GetConsistency() == nil {
		req.Consistency = &kesselapi.Consistency{Requirement: &kesselapi.Consistency_MinimizeLatency{MinimizeLatency: true}}
	}

	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckBulk")
		return nil, err
	}

	resp, err := a.CheckService.CheckBulk(ctx, &kesselapi.CheckBulkRequest{
		Items:       req.GetItems(),
		Consistency: req.GetConsistency(),
	}, opts...)
	if err != nil {
		a.incrFailureCounter("CheckBulk")
		return nil, err
	}

	a.incrSuccessCounter("CheckBulk")
	return resp, nil
}

func (a *GRPCRelationsRepository) CheckForUpdateBulk(ctx context.Context, req *kesselapi.CheckForUpdateBulkRequest) (*kesselapi.CheckForUpdateBulkResponse, error) {
	log.Infof("CheckForUpdateBulk: checking %d items", len(req.GetItems()))
	opts, err := a.getCallOptions()
	if err != nil {
		a.incrFailureCounter("CheckForUpdateBulk")
		return nil, err
	}
	resp, err := a.CheckService.CheckForUpdateBulk(ctx, req, opts...)
	if err != nil {
		a.incrFailureCounter("CheckForUpdateBulk")
		return nil, err
	}
	a.incrSuccessCounter("CheckForUpdateBulk")
	return resp, nil
}

// SetWorkspace upsert inserts the relationship in relations if it doesn't exist and otherwise does nothing
func (a *GRPCRelationsRepository) SetWorkspace(ctx context.Context, local_resource_id, workspace, namespace, name string, upsert bool) (*kesselapi.CreateTuplesResponse, error) {
	if workspace == "" {
		err := fmt.Errorf("workspace_id is required")
		a.incrFailureCounter("SetWorkspace")
		return nil, err
	}
	rels := []*kesselapi.Relationship{{
		Resource: &kesselapi.ObjectReference{
			Type: &kesselapi.ObjectType{
				Name:      name,
				Namespace: namespace,
			},
			Id: local_resource_id,
		},
		Relation: "workspace",
		Subject: &kesselapi.SubjectReference{
			Subject: &kesselapi.ObjectReference{
				Type: &kesselapi.ObjectType{
					Name:      "workspace",
					Namespace: "rbac",
				},
				Id: workspace,
			},
		},
	}}

	a.incrSuccessCounter("SetWorkspace")
	return a.CreateTuples(ctx, &kesselapi.CreateTuplesRequest{
		Upsert: upsert,
		Tuples: rels,
	})
}
