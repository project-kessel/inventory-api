package sqlite3

import "github.com/google/wire"

var ProviderSet = wire.NewSet(NewOptions, NewConfig, NewCompleteConfig)
