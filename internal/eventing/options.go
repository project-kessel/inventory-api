package eventing

import (
	"errors"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/eventing/kafka"
)

type Options struct {
	Kafka   *kafka.Options `mapstructure:"kafka"`
	Eventer string         `mapstructure:"eventer"`
}

func NewOptions() *Options {
	return &Options{
		Kafka:   kafka.NewOptions(),
		Eventer: "stdout",
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Eventer, prefix+"eventer", o.Eventer, "The eventing subsystem to use.  Either stdout or kafka.")

	o.Kafka.AddFlags(fs, prefix+"kafka")
}

func (o *Options) Complete() []error {
	return nil
}

func (o *Options) Validate() []error {
	var errs []error
	if o.Eventer != "stdout" && o.Eventer != "kafka" {
		errs = append(errs, errors.New("eventer must be either stdout or kafka"))
	}

	if o.Eventer == "kafka" {
		errs = append(errs, o.Kafka.Validate()...)
	}

	return errs
}
