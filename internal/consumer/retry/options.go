package retry

import "github.com/spf13/pflag"

type Options struct {
	ConsumerMaxRetries  int `mapstructure:"consumer-max-retries"`
	OperationMaxRetries int `mapstructure:"operation-max-retries"`
	BackoffFactor       int `mapstructure:"backoff-factor"`
}

func NewOptions() *Options {
	return &Options{
		ConsumerMaxRetries:  2,
		OperationMaxRetries: 3,
		BackoffFactor:       5,
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.IntVar(&o.ConsumerMaxRetries, prefix+"consumer-max-retries", o.ConsumerMaxRetries, "sets the bootstrap server address and port for Kafka")
	fs.IntVar(&o.OperationMaxRetries, prefix+"operation-max-retries", o.OperationMaxRetries, "sets the Kafka consumer group name (default: inventory-consumer)")
	fs.IntVar(&o.BackoffFactor, prefix+"backoff-factor", o.BackoffFactor, "Kafka topic to monitor for events")
}
