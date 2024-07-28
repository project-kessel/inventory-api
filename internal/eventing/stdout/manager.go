package stdout

import (
	"context"
	"encoding/json"
	"os"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/models"
)

type StdOutManager struct {
	Encoder *json.Encoder
	Errors  chan error
}

func New() (*StdOutManager, error) {
	return &StdOutManager{
		Encoder: json.NewEncoder(os.Stdout),
		Errors:  make(chan error),
	}, nil
}

func (p *StdOutManager) Produce(ctx context.Context, event *api.Event) error {
	return p.Encoder.Encode(event)
}

func (m *StdOutManager) Errs() <-chan error {
	return m.Errors
}

// Lookup figures out which Producer should be used for the given identity and resource.
func (m *StdOutManager) Lookup(identity *authnapi.Identity, resource *models.Resource) (api.Producer, error) {
	return m, nil
}

func (m *StdOutManager) Shutdown(ctx context.Context) error {
	return nil
}
