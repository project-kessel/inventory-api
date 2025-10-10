package pprof

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof" // Import pprof to register its handlers
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

const (
	DefaultReadTimeout  = 15 * time.Second
	DefaultWriteTimeout = 15 * time.Second
)

// Server wraps the pprof HTTP server
type Server struct {
	server *http.Server
	logger *log.Helper
}

// New creates a new pprof server with the given options
func New(opts *Options, logger log.Logger) (*Server, error) {
	if !opts.Enabled {
		return nil, nil
	}

	helper := log.NewHelper(log.With(logger, "subsystem", "pprof"))

	mux := http.NewServeMux()
	// The pprof handlers are automatically registered to the default mux
	// We need to copy them to our custom mux
	mux.Handle("/debug/pprof/", http.DefaultServeMux)
	mux.Handle("/debug/pprof/cmdline", http.DefaultServeMux)
	mux.Handle("/debug/pprof/profile", http.DefaultServeMux)
	mux.Handle("/debug/pprof/symbol", http.DefaultServeMux)
	mux.Handle("/debug/pprof/trace", http.DefaultServeMux)

	server := &http.Server{
		Addr:         opts.GetListenAddr(),
		Handler:      mux,
		ReadTimeout:  DefaultReadTimeout,
		WriteTimeout: DefaultWriteTimeout,
	}

	return &Server{
		server: server,
		logger: helper,
	}, nil
}

// Start starts the pprof server
func (s *Server) Start() error {
	if s == nil {
		return nil
	}

	s.logger.Infof("Starting pprof server on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("pprof server error: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the pprof server
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}

	s.logger.Info("Shutting down pprof server")
	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("error shutting down pprof server: %w", err)
	}
	return nil
}
