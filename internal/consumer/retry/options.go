package retry

import "github.com/spf13/pflag"

type Options struct {
	ConsumerMaxRetries  string `mapstructure:"consumer-max-retries"`
	OperationMaxRetries string `mapstructure:"operation-max-retries"`
	BackoffFactor       string `mapstructure:"backoff-factor"`
}

func NewOptions() *Options {
	return &Options{
		ConsumerMaxRetries:  "3",
		OperationMaxRetries: "3",
		BackoffFactor:       "4",
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.StringVar(&o.ConsumerMaxRetries, prefix+"consumer-max-retries", o.ConsumerMaxRetries, "sets the bootstrap server address and port for Kafka")
	fs.StringVar(&o.OperationMaxRetries, prefix+"operation-max-retries", o.OperationMaxRetries, "sets the Kafka consumer group name (default: inventory-consumer)")
	fs.StringVar(&o.BackoffFactor, prefix+"backoff-factor", o.BackoffFactor, "Kafka topic to monitor for events")
}
