package eventing

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/eventing/kafka"
	"github.com/project-kessel/inventory-api/internal/eventing/stdout"
)

func New(c CompletedConfig, source string, logger *log.Helper) (api.Manager, error) {
	switch c.Eventer {
	case "stdout":
		return stdout.New(logger)
	case "kafka":
		km, err := kafka.New(c.Kafka, source, logger)
		if err != nil {
			return nil, err
		}
		return km, nil
	}

	return nil, fmt.Errorf("unrecognized eventer type: %s", c.Eventer)
}
