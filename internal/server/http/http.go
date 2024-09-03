package http

import (
	"github.com/bufbuild/protovalidate-go"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/middleware/selector"
	"github.com/go-kratos/kratos/v2/transport/http"
	m "github.com/project-kessel/inventory-api/internal/middleware"
)

// New create a new http server.
func New(c CompletedConfig, authn middleware.Middleware) *http.Server {
	validator, err := protovalidate.New()
	if err != nil {
		return nil
	}
	// TODO: pass in health, authn middleware
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			m.Validation(validator),
			selector.Server(
				authn,
			).Match(NewWhiteListMatcher).Build(),
		),
	}
	opts = append(opts, c.ServerOptions...)
	srv := http.NewServer(opts...)
	return srv
}
