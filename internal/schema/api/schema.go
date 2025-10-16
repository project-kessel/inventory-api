package api

import (
	"context"
)

type Resource struct {
	ResourceType string
	CommonSchema string
}

type ResourceReporter struct {
	ResourceType   string
	ReporterType   string
	ReporterSchema string
}

type SchemaService interface {
	ValidateReporterForResource(ctx context.Context, resourceType string, reporterType string) error
	CommonShallowValidate(ctx context.Context, resourceType string, commonRepresentation map[string]interface{}) error
	ReporterShallowValidate(ctx context.Context, resourceType string, reporterType string, reporterRepresentation map[string]interface{}) error
}

type SchemaRepository interface {
	GetResources(ctx context.Context) ([]string, error)
	CreateResource(ctx context.Context, resource Resource) error
	GetResource(ctx context.Context, resourceType string) (Resource, error)
	UpdateResource(ctx context.Context, resource Resource) error
	DeleteResource(ctx context.Context, resourceType string) error

	GetResourceReporters(ctx context.Context, resourceType string) ([]string, error)
	CreateResourceReporter(ctx context.Context, resourceReporter ResourceReporter) error
	GetResourceReporter(ctx context.Context, resourceType string, reporterType string) (ResourceReporter, error)
	UpdateResourceReporter(ctx context.Context, resourceReporter ResourceReporter) error
	DeleteResourceReporter(ctx context.Context, resourceType string, reporterType string) error
}
