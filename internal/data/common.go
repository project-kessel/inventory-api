package data

import (
	"context"
	authzapi "github.com/project-kessel/inventory-api/internal/authz/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"time"
)

func DefaultResourceSendEvent(ctx context.Context, model *model.Resource, eventer eventingapi.Manager, reportedTime time.Time, operationType eventingapi.OperationType) error {
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

func DefaultRelationshipSendEvent(ctx context.Context, model *model.Relationship, eventer eventingapi.Manager, operationType eventingapi.OperationType) error {
	identity, err := middleware.GetIdentity(ctx)
	if err != nil {
		return err
	}

	producer, _ := eventer.Lookup(identity, model.RelationshipType, model.ID)
	evt, err := eventingapi.NewRelationshipEvent(operationType, model, time.Now())
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
	_, err := authz.SetWorkspace(ctx, model.Reporter.LocalResourceId, model.Workspace, namespace, model.ResourceType)
	if err != nil {
		return err
	}

	return nil
}
