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
	kratosMiddleware "github.com/go-kratos/kratos/v2/middleware"
	"github.com/project-kessel/inventory-api/internal/metricscollector"
	"github.com/project-kessel/inventory-api/internal/service"
	"github.com/sony/gobreaker"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"gorm.io/gorm"

	"github.com/project-kessel/inventory-api/cmd/common"
	resourcesctl "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/config/schema"
	"github.com/project-kessel/inventory-api/internal/consistency"
	"github.com/project-kessel/inventory-api/internal/consumer"
	"github.com/project-kessel/inventory-api/internal/data"
	inventoryResourcesRepo "github.com/project-kessel/inventory-api/internal/data/inventoryresources"
	legacyresourcerepo "github.com/project-kessel/inventory-api/internal/data/resources"
	"github.com/project-kessel/inventory-api/internal/pubsub"

	//v1beta2
	resourcesvc "github.com/project-kessel/inventory-api/internal/service/resources"

	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/errors"
	"github.com/project-kessel/inventory-api/internal/eventing"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/server/pprof"
	"github.com/project-kessel/inventory-api/internal/storage"

	hb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	healthctl "github.com/project-kessel/inventory-api/internal/biz/health"
	healthrepo "github.com/project-kessel/inventory-api/internal/data/health"
	healthssvc "github.com/project-kessel/inventory-api/internal/service/health"
)

// NewCommand creates a new cobra command for starting the inventory server.
// It configures and wires together all the necessary components including storage, authentication,
// authorization, eventing, and consumer services.
func NewCommand(
	serverOptions *server.Options,
	storageOptions *storage.Options,
	authnOptions *authn.Options,
	authzOptions *authz.Options,
	eventingOptions *eventing.Options,
	consumerOptions *consumer.Options,
	consistencyOptions *consistency.Options,
	serviceOptions *service.Options,
	loggerOptions common.LoggerOptions,
	schemaOptions *schema.Options,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the inventory server",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, logger := common.InitLogger(common.GetLogLevel(), loggerOptions)
			ctx := context.Background()

			var consumerConfig consumer.CompletedConfig
			var inventoryConsumer consumer.InventoryConsumer

			// configure storage
			if errs := storageOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := storageOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			storageConfig := storage.NewConfig(storageOptions).Complete()

			// // configure authn
			if errs := authnOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := authnOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			authnConfig, errs := authn.NewConfig(authnOptions).Complete()
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// configure authz
			if errs := authzOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := authzOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			authzConfig, errs := authz.NewConfig(authzOptions).Complete(ctx)
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// configure eventing
			if errs := eventingOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := eventingOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			eventingConfig, errs := eventing.NewConfig(eventingOptions).Complete()
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// configure inventoryConsumer
			if errs := consumerOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := consumerOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if consumerOptions.Enabled {
				consumerConfig, errs = consumer.NewConfig(consumerOptions).Complete()
				if errs != nil {
					return errors.NewAggregate(errs)
				}
			}

			// configure consistency
			if errs := consistencyOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := consistencyOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			consistencyConfig, errs := consistency.NewConfig(consistencyOptions).Complete()
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// configure schemaService service
			if errs := schemaOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := schemaOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			schemaConfig, errs := schema.NewConfig(schemaOptions).Complete()
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// configure the server
			if errs := serverOptions.Complete(); errs != nil {
				return errors.NewAggregate(errs)
			}
			if errs := serverOptions.Validate(); errs != nil {
				return errors.NewAggregate(errs)
			}
			serverConfig, errs := server.NewConfig(serverOptions).Complete()
			if errs != nil {
				return errors.NewAggregate(errs)
			}

			// construct storage
			db, err := storage.New(storageConfig, log.NewHelper(log.With(logger, "subsystem", "storage")))
			if err != nil {
				return err
			}

			// setup metrics collector for consumer and custom metrics
			mc := &metricscollector.MetricsCollector{}
			meter := otel.Meter("github.com/project-kessel/inventory-api/blob/main/internal/server/otel")
			err = mc.New(meter)
			if err != nil {
				return err
			}

			// START: construct pubsub (postgres only)
			var listenManager *pubsub.ListenManager
			var notifier *pubsub.PgxNotifier
			listenManagerErr := make(chan error)
			if storageConfig.Options.Database == "postgres" {
				pubSubLogger := log.NewHelper(log.With(logger, "subsystem", "pubsub"))
				pgxPool, err := storage.NewPgx(storageConfig, pubSubLogger)
				if err != nil {
					return err
				}
				listenerDriver := pubsub.NewPgxDriver(pgxPool)
				if err := listenerDriver.Connect(ctx); err != nil {
					return fmt.Errorf("error setting up listenerDriver: %v", err)
				}
				err = listenerDriver.Listen(ctx)
				if err != nil {
					return fmt.Errorf("error setting up listener: %v", err)
				}
				listenManager = pubsub.NewListenManager(pubSubLogger, listenerDriver)

				go func() {
					listenManagerErr <- listenManager.Run(ctx)
				}()

				// Run notifier on a separate connection, as the listener requires it's own
				notifierDriver := pubsub.NewPgxDriver(pgxPool)
				if err := notifierDriver.Connect(ctx); err != nil {
					return fmt.Errorf("error setting up notifierDriver: %v", err)
				}
				notifier = pubsub.NewPgxNotifier(notifierDriver)
			}
			// STOP: construct pubsub

			// construct authn
			authenticator, err := authn.New(authnConfig, log.NewHelper(log.With(logger, "subsystem", "authn")))
			if err != nil {
				return err
			}

			// construct authz
			authorizer, err := authz.New(ctx, authzConfig, log.NewHelper(log.With(logger, "subsystem", "authz")))
			if err != nil {
				return err
			}

			// Configure meta-authorizer middleware from config
			var metaAuthorizerMiddleware kratosMiddleware.Middleware
			if authzConfig.MetaAuthorizer != nil && authzConfig.MetaAuthorizer.Enabled {
				metaAuthorizerConfig := middleware.MetaAuthorizerConfig{
					Authorizer: authorizer,
					Namespace:  authzConfig.MetaAuthorizer.Namespace,
					Enabled:    authzConfig.MetaAuthorizer.Enabled,
				}
				metaAuthorizerMiddleware = middleware.MetaAuthorizerMiddleware(metaAuthorizerConfig, logger)
			} else {
				// Create a no-op middleware if meta-authorizer is disabled
				metaAuthorizerMiddleware = func(next kratosMiddleware.Handler) kratosMiddleware.Handler {
					return next
				}
			}

			// construct eventing
			// Note that we pass the server id here to act as the Source URI in cloudevents
			// If a server ID isn't configured explicitly, `os.Hostname()` is used.
			eventingManager, err := eventing.New(eventingConfig, serverConfig.Options.PublicUrl, log.NewHelper(log.With(logger, "subsystem", "eventing")))
			if err != nil {
				return err
			}

			// constructs schema repository
			schemaRepository, err := data.NewSchemaRepository(ctx, schemaConfig, log.NewHelper(log.With(logger, "subsystem", "schemaRepository")))
			if err != nil {
				return err
			}

			// construct servers
			server, err := server.New(serverConfig, middleware.Authentication(authenticator), metaAuthorizerMiddleware, authnConfig, authenticator, logger)
			if err != nil {
				return err
			}

			// construct pprof server
			pprofServer, err := pprof.New(serverConfig.Options.PprofOptions, logger)
			if err != nil {
				return err
			}

			inventoryresources_repo := inventoryResourcesRepo.New(db)

			usecaseConfig := &resourcesctl.UsecaseConfig{
				ReadAfterWriteEnabled:   consistencyConfig.ReadAfterWriteEnabled,
				ReadAfterWriteAllowlist: consistencyConfig.ReadAfterWriteAllowlist,
				ConsumerEnabled:         consumerOptions.Enabled,
			}

			// This circuit breaker is used to prevent request handlers from being blocked
			// indefinitely if the consumer is not responding via notifications.
			// This is a naive solution until we can implement a more robust
			// solution for navigating consumer health.
			waitForNotifCircuitBreaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
				Name:    "wait-for-notif-breaker",
				Timeout: 60 * time.Second, // Reset after 60s if tripped
				ReadyToTrip: func(counts gobreaker.Counts) bool {
					// Trip after 3 consecutive failures
					return counts.ConsecutiveFailures > 2
				},
				OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
					log.Debugf("Circuit breaker %s changed from %s to %s", name, from, to)
				},
			})

			// Create transaction manager for all repositories
			transactionManager := data.NewGormTransactionManager(mc, storageConfig.Options.MaxSerializationRetries)

			//v1beta2
			// wire together inventory service handling
			resourceRepo := data.NewResourceRepository(db, transactionManager)
			legacy_resource_repo := legacyresourcerepo.New(db, mc, transactionManager)
			inventory_controller := resourcesctl.New(resourceRepo, legacy_resource_repo, inventoryresources_repo, schemaRepository, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig, mc)

			inventory_service := resourcesvc.NewKesselInventoryServiceV1beta2(inventory_controller)
			pbv1beta2.RegisterKesselInventoryServiceServer(server.GrpcServer, inventory_service)
			pbv1beta2.RegisterKesselInventoryServiceHTTPServer(server.HttpServer, inventory_service)

			health_repo := healthrepo.New(db, authorizer, authzConfig)
			health_controller := healthctl.New(health_repo, log.With(logger, "subsystem", "health_controller"))
			health_service := healthssvc.New(health_controller)
			hb.RegisterKesselInventoryHealthServiceServer(server.GrpcServer, health_service)
			hb.RegisterKesselInventoryHealthServiceHTTPServer(server.HttpServer, health_service)

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

			shutdown := shutdown(db, server, pprofServer, eventingManager, &inventoryConsumer, log.NewHelper(logger))

			if consumerOptions.Enabled {
				go func() {
					retries := 0
					for consumerOptions.RetryOptions.ConsumerMaxRetries == -1 || retries < consumerOptions.RetryOptions.ConsumerMaxRetries {
						// If the consumer cannot process a message, the consumer loop is restarted
						// This is to ensure we re-read the message and prevent it being dropped and moving to next message.
						// To re-read the current message, we have to recreate the consumer connection so that the earliest offset is used
						inventoryConsumer, err = consumer.New(consumerConfig, db, schemaRepository, authzConfig, authorizer, notifier, log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer")), nil)
						if err != nil {
							shutdown(err)
						}
						err = inventoryConsumer.Consume()
						if e.Is(err, consumer.ErrClosed) {
							inventoryConsumer.Logger.Errorf("consumer unable to process current message -- restarting consumer")
							retries++
							if consumerOptions.RetryOptions.ConsumerMaxRetries == -1 || retries < consumerOptions.RetryOptions.ConsumerMaxRetries {
								backoff := min(time.Duration(inventoryConsumer.RetryOptions.BackoffFactor*retries*300)*time.Millisecond, time.Duration(consumerOptions.RetryOptions.MaxBackoffSeconds)*time.Second)
								inventoryConsumer.Logger.Errorf("retrying in %v", backoff)
								time.Sleep(backoff)
							}
							continue
						} else {
							inventoryConsumer.Logger.Errorf("consumer unable to process messages: %v", err)
							shutdown(err)
						}
					}
					shutdown(fmt.Errorf("consumer unable to process current message -- max retries reached"))
				}()
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			select {
			case err := <-srvErrs:
				shutdown(err)
			case pprofErr := <-pprofErrs:
				shutdown(pprofErr)
			case lmErr := <-listenManagerErr:
				shutdown(lmErr)
			case sig := <-quit:
				shutdown(sig)
			case emErr := <-eventingManager.Errs():
				shutdown(emErr)
			case cmErr := <-inventoryConsumer.Errs():
				shutdown(cmErr)
			}
			return nil
		},
	}

	serverOptions.AddFlags(cmd.Flags(), "server")
	authnOptions.AddFlags(cmd.Flags(), "authn")
	authzOptions.AddFlags(cmd.Flags(), "authz")
	eventingOptions.AddFlags(cmd.Flags(), "eventing")
	consumerOptions.AddFlags(cmd.Flags(), "consumer")
	consistencyOptions.AddFlags(cmd.Flags(), "consistency")
	serviceOptions.AddFlags()
	schemaOptions.AddFlags(cmd.Flags(), "schema")

	return cmd
}

// shutdown returns a shutdown function that gracefully closes all server components
// including the HTTP server, pprof server, eventing manager, consumer, and database connections.
func shutdown(db *gorm.DB, srv *server.Server, pprofSrv *pprof.Server, em eventingapi.Manager, cm *consumer.InventoryConsumer, logger *log.Helper) func(reason interface{}) {
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

		if cm != nil {
			defer func() {
				err := cm.Shutdown()
				if err != nil {
					if e.Is(err, consumer.ErrClosed) {
						logger.Warn("error shutting down consumer, consumer already closed")
					} else {
						logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Consumer: %v", err))
					}
				}
			}()
		}

		if sqlDB, err := db.DB(); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Storage: %v", err))
		} else {
			defer func() {
				if err := sqlDB.Close(); err != nil {
					fmt.Printf("failed to close consumer: %v", err)
				}
			}()
		}
	}
}
