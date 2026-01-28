package serve

import (
	"context"
	e "errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/sony/gobreaker"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/internal/biz/model"
	"github.com/project-kessel/inventory-api/internal/biz/usecase/replication"
	resourcesctl "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/data"
	"github.com/project-kessel/inventory-api/internal/errors"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/inventory-api/internal/provider"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/server/pprof"

	hb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	healthctl "github.com/project-kessel/inventory-api/internal/biz/health"
	healthrepo "github.com/project-kessel/inventory-api/internal/data/health"
	healthssvc "github.com/project-kessel/inventory-api/internal/service/health"

	//v1beta2
	resourcesvc "github.com/project-kessel/inventory-api/internal/service/resources"
)

// NewCommand creates a new cobra command for starting the inventory server.
// It configures and wires together all the necessary components including storage, authentication,
// authorization, eventing, and consumer services.
func NewCommand(
	options *provider.Options,
	loggerOptions common.LoggerOptions,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the inventory server",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
			ctx := context.Background()

			// Validate all options
			if err := validateOptions(options); err != nil {
				return err
			}

			// Create storage
			storageLogger := log.NewHelper(log.With(logger, "subsystem", "storage"))
			db, err := provider.NewStorage(options.Storage, storageLogger)
			if err != nil {
				return err
			}

			// Create pgx pool for postgres-specific features
			pgxPool, err := provider.NewPgxPool(ctx, options.Storage, storageLogger)
			if err != nil {
				return err
			}

			// Create cluster broadcast (postgres only)
			clusterBroadcast, err := provider.NewClusterBroadcast(ctx, options.Storage, pgxPool, logger)
			if err != nil {
				return err
			}

			// Create authenticator
			authnLogger := log.NewHelper(log.With(logger, "subsystem", "authn"))
			authnResult, err := provider.NewAuthenticator(options.Authn, authnLogger)
			if err != nil {
				return err
			}

			// Create authorizer
			authzLogger := log.NewHelper(log.With(logger, "subsystem", "authz"))
			authorizer, err := provider.NewAuthorizer(ctx, options.Authz, authzLogger)
			if err != nil {
				return err
			}

			// Create eventing manager
			eventingLogger := log.NewHelper(log.With(logger, "subsystem", "eventing"))
			eventingManager, err := provider.NewEventingManager(options.Eventing, options.Server.PublicUrl, eventingLogger)
			if err != nil {
				return err
			}

			// Create schema repository
			schemaLogger := log.NewHelper(log.With(logger, "subsystem", "schemaRepository"))
			schemaRepository, err := provider.NewSchemaRepository(ctx, options.Schema, schemaLogger)
			if err != nil {
				return err
			}

			// Create server
			server, err := provider.NewServer(provider.ServerConfig{
				Options:         options.Server,
				AuthnMiddleware: middleware.Authentication(authnResult.Authenticator),
				Authenticator:   authnResult.Authenticator,
				Logger:          logger,
			})
			if err != nil {
				return err
			}

			// Create pprof server - convert provider options to pprof options
			pprofOpts := &pprof.Options{
				Enabled: options.Server.Pprof.Enabled,
				Port:    options.Server.Pprof.Port,
				Addr:    options.Server.Pprof.Addr,
			}
			pprofServer, err := pprof.New(pprofOpts, logger)
			if err != nil {
				return err
			}

			// Setup metrics collector
			mc := &metricscollector.MetricsCollector{}
			meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
			err = mc.New(meter)
			if err != nil {
				return err
			}

			// Build consistency config
			consistencyConfig := provider.BuildConsistencyConfig(options.Consistency)

			usecaseConfig := &resourcesctl.UsecaseConfig{
				ReadAfterWriteEnabled:   consistencyConfig.ReadAfterWriteEnabled,
				ReadAfterWriteAllowlist: consistencyConfig.ReadAfterWriteAllowlist,
				ConsumerEnabled:         options.Consumer.Enabled,
			}

			// Circuit breaker for wait-for-notification
			waitForNotifCircuitBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:    "wait-for-notif-breaker",
				Timeout: 60 * time.Second,
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					return counts.ConsecutiveFailures > 2
				},
				OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
					log.Debugf("Circuit breaker %s changed from %s to %s", name, from, to)
				},
			})

			// Create transaction manager
			transactionManager := data.NewGormTransactionManager(mc, options.Storage.MaxSerializationRetries)

			// Create resource repository
			resourceRepo := data.NewResourceRepository(db, transactionManager)

			// Create event source if consumer is enabled
			var eventSource model.EventSource
			if options.Consumer.Enabled {
				eventSource, err = provider.NewKafkaEventSource(options.Consumer, authorizer, log.With(logger, "subsystem", "kafkaEventSource"))
				if err != nil {
					return err
				}
			}

			// Create store for transactional access
			store := data.NewAdapterStore(data.AdapterStoreConfig{
				ResourceRepo: resourceRepo,
				EventSource:  eventSource,
			})

			// Create inventory controller
			inventoryController := resourcesctl.New(store, clusterBroadcast, schemaRepository, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), waitForNotifCircuitBreaker, usecaseConfig, mc)

			// Create replication usecase if consumer is enabled
			var replicationUsecase *replication.RelationReplicationUsecase
			if options.Consumer.Enabled {
				schemaService := model.NewSchemaService(schemaRepository, log.NewHelper(log.With(logger, "subsystem", "schemaService")))
				relationsReplicator := newAuthorizerReplicator(authorizer)
				replicationService := model.NewRelationReplicationService(relationsReplicator, schemaService)
				replicationUsecase = replication.NewRelationReplicationUsecase(
					store,
					clusterBroadcast,
					replicationService,
					log.With(logger, "subsystem", "replicationUsecase"),
				)
			}

			// Register services
			inventoryService := resourcesvc.NewKesselInventoryServiceV1beta2(inventoryController)
			pbv1beta2.RegisterKesselInventoryServiceServer(server.GrpcServer, inventoryService)
			pbv1beta2.RegisterKesselInventoryServiceHTTPServer(server.HttpServer, inventoryService)

			healthRepo := healthrepo.New(db, authorizer)
			healthController := healthctl.New(healthRepo, log.With(logger, "subsystem", "health_controller"))
			healthService := healthssvc.New(healthController)
			hb.RegisterKesselInventoryHealthServiceServer(server.GrpcServer, healthService)
			hb.RegisterKesselInventoryHealthServiceHTTPServer(server.HttpServer, healthService)

			// Start servers
			srvErrs := make(chan error)
			go func() {
				srvErrs <- server.Run(ctx)
			}()

			pprofErrs := make(chan error)
			if pprofServer != nil {
				go func() {
					pprofErrs <- pprofServer.Start()
				}()
			}

			shutdown := createShutdown(db, server, pprofServer, eventingManager, log.NewHelper(logger))

			// Start replication usecase if enabled
			replicationErrs := make(chan error, 1)
			if options.Consumer.Enabled && replicationUsecase != nil {
				go func() {
					if err := replicationUsecase.Run(ctx); err != nil && !e.Is(err, context.Canceled) {
						replicationErrs <- err
					}
				}()
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			select {
			case err := <-srvErrs:
				shutdown(err)
			case pprofErr := <-pprofErrs:
				shutdown(pprofErr)
			case sig := <-quit:
				shutdown(sig)
			case emErr := <-eventingManager.Errs():
				shutdown(emErr)
			case repErr := <-replicationErrs:
				shutdown(repErr)
			}
			return nil
		},
	}

	// Add flags for each option group
	options.Server.AddFlags(cmd.Flags(), "server")
	options.Authn.AddFlags(cmd.Flags(), "authn")
	options.Authz.AddFlags(cmd.Flags(), "authz")
	options.Eventing.AddFlags(cmd.Flags(), "eventing")
	options.Consumer.AddFlags(cmd.Flags(), "consumer")
	options.Consistency.AddFlags(cmd.Flags(), "consistency")
	options.Schema.AddFlags(cmd.Flags(), "schema")

	return cmd
}

// validateOptions validates all options before use.
func validateOptions(opts *provider.Options) error {
	var allErrs []error

	if errs := opts.Storage.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}
	if errs := opts.Authn.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}
	if errs := opts.Authz.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}
	if errs := opts.Eventing.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}
	if opts.Consumer.Enabled {
		if errs := opts.Consumer.Validate(); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}
	if errs := opts.Server.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}
	if errs := opts.Schema.Validate(); len(errs) > 0 {
		allErrs = append(allErrs, errs...)
	}

	if len(allErrs) > 0 {
		return errors.NewAggregate(allErrs)
	}
	return nil
}

// createShutdown returns a shutdown function that gracefully closes all server components.
func createShutdown(db *gorm.DB, srv *server.Server, pprofSrv *pprof.Server, em eventingapi.Manager, logger *log.Helper) func(reason interface{}) {
	return func(reason interface{}) {
		log.Info(fmt.Sprintf("Server Shutdown: %s", reason))

		timeout := srv.HttpServer.ReadTimeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down API: %v", err))
		}

		if pprofSrv != nil {
			ctx, cancel = context.WithTimeout(context.Background(), timeout)
			defer cancel()
			if err := pprofSrv.Shutdown(ctx); err != nil {
				logger.Error(fmt.Sprintf("Error Gracefully Shutting Down pprof: %v", err))
			}
		}

		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := em.Shutdown(ctx); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Eventing: %v", err))
		}

		if sqlDB, err := db.DB(); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Storage: %v", err))
		} else {
			defer func() {
				if err := sqlDB.Close(); err != nil {
					fmt.Printf("failed to close database: %v", err)
				}
			}()
		}
	}
}

// newAuthorizerReplicator creates a relations replicator from the authorizer.
// This is a bridge to use authz.NewAuthorizerReplicator without importing the authz package directly.
func newAuthorizerReplicator(authorizer interface{}) model.RelationsReplicator {
	// Import is avoided to prevent circular dependency; we use the interface.
	// The authorizer returned from provider.NewAuthorizer already satisfies the required interface.
	if replicator, ok := authorizer.(model.RelationsReplicator); ok {
		return replicator
	}
	// Fallback: create from authz package
	// This shouldn't happen as authorizer should implement the interface
	return nil
}

