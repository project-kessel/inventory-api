package otel

// Taken from Kratos examples: https://github.com/go-kratos/examples/blob/main/otel/internal/dep/otel.go

import (
	a "github.com/go-kratos/kratos/v2/middleware/metrics"
	"github.com/project-kessel/inventory-api/internal/server"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

func NewMeter(c server.CompletedConfig, provider metric.MeterProvider) (metric.Meter, error) {
	return provider.Meter("inventory-api"), nil
}

func NewMeterProvider(c server.CompletedConfig) (metric.MeterProvider, error) {
	exporter, err := prometheus.New()
	if err != nil {
		return nil, err
	}

	const pathPrefix = "/api/inventory"
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(
			resource.NewWithAttributes(
				semconv.SchemaURL,
				semconv.ServiceNameKey.String("inventory-api"),
				attribute.String("environment", pathPrefix),
			),
		),
		sdkmetric.WithReader(exporter),
		sdkmetric.WithView(
			a.DefaultSecondsHistogramView(a.DefaultServerSecondsHistogramName),
		),
	)
	otel.SetMeterProvider(provider)
	return provider, nil
}
