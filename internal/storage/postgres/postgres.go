package postgres

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewOptions, NewConfig, NewCompleteConfig)
