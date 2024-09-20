package server

import (
	"os"

	"github.com/spf13/pflag"

	"github.com/project-kessel/inventory-api/internal/server/grpc"
	"github.com/project-kessel/inventory-api/internal/server/http"
)

type Options struct {
	Id        string `mapstructure:"id"`
	Name      string `mapstructure:"name"`
	PublicUrl string `mapstructure:"public_url"`

	GrpcOptions *grpc.Options `mapstructure:"grpc"`
	HttpOptions *http.Options `mapstructure:"http"`
}

func NewOptions() *Options {
	id, _ := os.Hostname()
	return &Options{
		Id:        id,
		Name:      "kessel-asset-inventory",
		PublicUrl: "http://localhost:8081",

		GrpcOptions: grpc.NewOptions(),
		HttpOptions: http.NewOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}

	fs.StringVar(&o.Id, prefix+"id", o.Id, "id of the server")
	fs.StringVar(&o.Name, prefix+"name", o.Name, "name of the server")
	fs.StringVar(&o.PublicUrl, prefix+"public_url", o.PublicUrl, "Public url where the server is reachable")

	o.GrpcOptions.AddFlags(fs, prefix+"grpc")
	o.HttpOptions.AddFlags(fs, prefix+"http")
}

func (o *Options) Complete() []error {
	var errors []error

	errors = append(errors, o.GrpcOptions.Complete()...)
	errors = append(errors, o.HttpOptions.Complete()...)

	return errors
}

func (o *Options) Validate() []error {
	var errors []error

	errors = append(errors, o.GrpcOptions.Validate()...)
	errors = append(errors, o.HttpOptions.Validate()...)

	return errors
}
