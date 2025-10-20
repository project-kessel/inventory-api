package api

import (
	"context"
)

type ResourceSchema struct {
	ResourceType               string
	CommonRepresentationSchema string
}

type ReporterSchema struct {
	ResourceType                 string
	ReporterType                 string
	ReporterRepresentationSchema string
}

type SchemaRepository interface {
	GetResources(ctx context.Context) ([]string, error)
	CreateResource(ctx context.Context, resource ResourceSchema) error
	GetResource(ctx context.Context, resourceType string) (ResourceSchema, error)
	UpdateResource(ctx context.Context, resource ResourceSchema) error
	DeleteResource(ctx context.Context, resourceType string) error

	GetReporters(ctx context.Context, resourceType string) ([]string, error)
	CreateReporter(ctx context.Context, resourceReporter ReporterSchema) error
	GetReporter(ctx context.Context, resourceType string, reporterType string) (ReporterSchema, error)
	UpdateReporter(ctx context.Context, resourceReporter ReporterSchema) error
	DeleteReporter(ctx context.Context, resourceType string, reporterType string) error
}
