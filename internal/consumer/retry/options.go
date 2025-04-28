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
	fs.IntVar(&o.ConsumerMaxRetries, prefix+"consumer-max-retries", o.ConsumerMaxRetries, "sets the max number of retries to process a message before killing consumer (default: 3)")
	fs.IntVar(&o.OperationMaxRetries, prefix+"operation-max-retries", o.OperationMaxRetries, "sets the max number of retries to execute a request before failing out (default: 4)")
	fs.IntVar(&o.BackoffFactor, prefix+"backoff-factor", o.BackoffFactor, "value used to calculate backoff between requests/restarts (default: 4)")
}
