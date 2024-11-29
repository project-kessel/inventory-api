package eventing

import (
	"github.com/project-kessel/inventory-api/eventing/kafka"
)

type Config struct {
	Eventer string
	Kafka   *kafka.Config
}

type completedConfig struct {
	Eventer string
	Kafka   kafka.CompletedConfig
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	cfg := &Config{
		Eventer: o.Eventer,
	}

	if o.Eventer == "kafka" {
		cfg.Kafka = kafka.NewConfig(o.Kafka)
	}

	return cfg
}

func (c *Config) Complete() (CompletedConfig, []error) {
	cfg := &completedConfig{
		Eventer: c.Eventer,
	}

	if c.Eventer == "kafka" {
		if k, err := c.Kafka.Complete(); err != nil {
			return CompletedConfig{}, nil
		} else {
			cfg.Kafka = k
		}
	}

	return CompletedConfig{cfg}, nil
}
