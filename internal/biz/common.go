package biz

import (
	"context"
	"github.com/project-kessel/inventory-api/eventing/api"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"time"
)

func DefaultResourceSendEvent(ctx context.Context, model *model.Resource, eventer api.Manager, reportedTime time.Time, operationType api.OperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, model.ResourceType, model.ID)
	evt, err := api.NewResourceEvent(operationType, model, reportedTime)
	if err != nil {
		return err
	}

	err = producer.Produce(ctx, evt)
	if err != nil {
		return err
	}

	return nil
}

func DefaultRelationshipSendEvent(ctx context.Context, m *model.Relationship, eventer api.Manager, reportedTime time.Time, operationType api.OperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, m.RelationshipType, m.ID)
	evt, err := api.NewRelationshipEvent(operationType, m, reportedTime)
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
	_, err := authz.SetWorkspace(ctx, model.Reporter.LocalResourceId, model.WorkspaceId, namespace, model.ResourceType)
	if err != nil {
		return err
	}

	return nil
}
