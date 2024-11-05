package stdout

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"os"

	"github.com/go-kratos/kratos/v2/log"
	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/eventing/api"
)

type StdOutManager struct {
	Encoder *json.Encoder
	Errors  chan error

	Logger *log.Helper
}

func New(logger *log.Helper) (*StdOutManager, error) {
	logger.Info("Using eventing: stdout")
	return &StdOutManager{
		Encoder: json.NewEncoder(os.Stdout),
		Errors:  make(chan error),

		Logger: logger,
	}, nil
}

func (p *StdOutManager) Produce(ctx context.Context, event *api.Event) error {
	return p.Encoder.Encode(event)
}

func (m *StdOutManager) Errs() <-chan error {
	return m.Errors
}

// Lookup figures out which Producer should be used for the given identity and resource.
func (m *StdOutManager) Lookup(identity *authnapi.Identity, resource_type string, resource_id uuid.UUID) (api.Producer, error) {
	return m, nil
}

func (m *StdOutManager) Shutdown(ctx context.Context) error {
	return nil
}
