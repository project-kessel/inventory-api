package biz

import (
	"context"
	"time"

	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
)

func DefaultResourceSendEvent(ctx context.Context, model *model.Resource, eventer eventingapi.Manager, reportedTime time.Time, operationType model.EventOperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, model.ResourceType, model.ID)
	evt, err := eventingapi.NewResourceEvent(operationType, model, reportedTime)
	if err != nil {
		return err
	}

	err = producer.Produce(ctx, evt)
	if err != nil {
		return err
	}

	return nil
}

func DefaultRelationshipSendEvent(ctx context.Context, m *model.Relationship, eventer eventingapi.Manager, reportedTime time.Time, operationType model.EventOperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, m.RelationshipType, m.ID)
	evt, err := eventingapi.NewRelationshipEvent(operationType, m, reportedTime)
	if err != nil {
		return err
	}

	err = producer.Produce(ctx, evt)
	if err != nil {
		return err
	}

	return nil
}

func DefaultSetWorkspace(ctx context.Context, namespace string, model *model.Resource, authz authzapi.Authorizer) error {
	_, err := authz.SetWorkspace(ctx, model.Reporter.LocalResourceId, model.WorkspaceId, namespace, model.ResourceType) //nolint:staticcheck
	if err != nil {
		return err
	}

	return nil
}

func DefaultUnsetWorkspace(ctx context.Context, namespace string, localResourceId string, resourceType string, authz authzapi.Authorizer) error {
	_, err := authz.UnsetWorkspace(ctx, localResourceId, namespace, resourceType)
	if err != nil {
		return err
	}

	return nil
}
