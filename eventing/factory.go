package eventing

import (
	"fmt"
	"github.com/project-kessel/inventory-api/eventing/api"
	"github.com/project-kessel/inventory-api/eventing/kafka"
	"github.com/project-kessel/inventory-api/eventing/stdout"

	"github.com/go-kratos/kratos/v2/log"
)

func New(c CompletedConfig, source string, logger *log.Helper) (api.Manager, error) {
	switch c.Eventer {
	case "stdout":
		return stdout.New(logger)
	case "kafka":
		km, err := kafka.New(c.Kafka, source, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to create kafka manager: %w", err)
		}
		return km, nil
	}

	return nil, fmt.Errorf("unrecognized eventer type: %s", c.Eventer)
}
