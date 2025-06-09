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
	relationshipsctl "github.com/project-kessel/inventory-api/internal/biz/usecase/relationships"
	resourcesctl "github.com/project-kessel/inventory-api/internal/biz/usecase/resources"
	"github.com/project-kessel/inventory-api/internal/consistency"
	"github.com/project-kessel/inventory-api/internal/consumer"
	inventoryResourcesRepo "github.com/project-kessel/inventory-api/internal/data/inventoryresources"
	relationshipsrepo "github.com/project-kessel/inventory-api/internal/data/relationships"
	resourcerepo "github.com/project-kessel/inventory-api/internal/data/resources"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	relationshipssvc "github.com/project-kessel/inventory-api/internal/service/relationships/k8spolicy"
	hostssvc "github.com/project-kessel/inventory-api/internal/service/resources/hosts"
	k8sclusterssvc "github.com/project-kessel/inventory-api/internal/service/resources/k8sclusters"
	k8spoliciessvc "github.com/project-kessel/inventory-api/internal/service/resources/k8spolicies"
	notifssvc "github.com/project-kessel/inventory-api/internal/service/resources/notificationsintegrations"

	//v1beta2
	resourcesvc "github.com/project-kessel/inventory-api/internal/service/resources"

	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/errors"
	"github.com/project-kessel/inventory-api/internal/eventing"
	eventingapi "github.com/project-kessel/inventory-api/internal/eventing/api"
	"github.com/project-kessel/inventory-api/internal/middleware"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/storage"

	hb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1"
	authzv1beta1 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/authz"
	rel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"
	pbv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
	healthctl "github.com/project-kessel/inventory-api/internal/biz/health"
	healthrepo "github.com/project-kessel/inventory-api/internal/data/health"
	healthssvc "github.com/project-kessel/inventory-api/internal/service/health"
)

func NewCommand(
	serverOptions *server.Options,
	storageOptions *storage.Options,
	authnOptions *authn.Options,
	authzOptions *authz.Options,
	eventingOptions *eventing.Options,
	consumerOptions *consumer.Options,
	consistencyOptions *consistency.Options,
	loggerOptions common.LoggerOptions,
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
			if !storageOptions.DisablePersistence {
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

			// construct eventing
			// Note that we pass the server id here to act as the Source URI in cloudevents
			// If a server ID isn't configured explicitly, `os.Hostname()` is used.
			eventingManager, err := eventing.New(eventingConfig, serverConfig.Options.PublicUrl, log.NewHelper(log.With(logger, "subsystem", "eventing")))
			if err != nil {
				return err
			}

			// construct servers
			server, err := server.New(serverConfig, middleware.Authentication(authenticator), authnConfig, logger)
			if err != nil {
				return err
			}

			inventoryresources_repo := inventoryResourcesRepo.New(db)

			usecaseConfig := &resourcesctl.UsecaseConfig{
				DisablePersistence:      storageConfig.Options.DisablePersistence,
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

			//v1beta2
			// wire together inventory service handling
			resource_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			inventory_controller := resourcesctl.New(resource_repo, inventoryresources_repo, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			inventory_service := resourcesvc.NewKesselInventoryServiceV1beta2(inventory_controller)
			pbv1beta2.RegisterKesselInventoryServiceServer(server.GrpcServer, inventory_service)
			pbv1beta2.RegisterKesselInventoryServiceHTTPServer(server.HttpServer, inventory_service)

			//v1beta1
			// wire together notificationsintegrations handling
			notifs_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			notifs_controller := resourcesctl.New(notifs_repo, inventoryresources_repo, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			notifs_service := notifssvc.NewKesselNotificationsIntegrationsServiceV1beta1(notifs_controller)
			pb.RegisterKesselNotificationsIntegrationServiceServer(server.GrpcServer, notifs_service)
			pb.RegisterKesselNotificationsIntegrationServiceHTTPServer(server.HttpServer, notifs_service)

			// wire together authz handling
			authz_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			authz_controller := resourcesctl.New(authz_repo, inventoryresources_repo, authorizer, eventingManager, "authz", log.With(logger, "subsystem", "authz_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			authz_service := resourcesvc.NewKesselCheckServiceV1beta1(authz_controller)
			authzv1beta1.RegisterKesselCheckServiceServer(server.GrpcServer, authz_service)
			authzv1beta1.RegisterKesselCheckServiceHTTPServer(server.HttpServer, authz_service)

			// wire together hosts handling
			hosts_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			hosts_controller := resourcesctl.New(hosts_repo, inventoryresources_repo, authorizer, eventingManager, "hbi", log.With(logger, "subsystem", "hosts_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			hosts_service := hostssvc.NewKesselRhelHostServiceV1beta1(hosts_controller)
			pb.RegisterKesselRhelHostServiceServer(server.GrpcServer, hosts_service)
			pb.RegisterKesselRhelHostServiceHTTPServer(server.HttpServer, hosts_service)

			// wire together k8sclusters handling
			k8sclusters_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			k8sclusters_controller := resourcesctl.New(k8sclusters_repo, inventoryresources_repo, authorizer, eventingManager, "acm", log.With(logger, "subsystem", "k8sclusters_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			k8sclusters_service := k8sclusterssvc.NewKesselK8SClusterServiceV1beta1(k8sclusters_controller)
			pb.RegisterKesselK8SClusterServiceServer(server.GrpcServer, k8sclusters_service)
			pb.RegisterKesselK8SClusterServiceHTTPServer(server.HttpServer, k8sclusters_service)

			// wire together k8spolicies handling
			k8spolicies_repo := resourcerepo.New(db, mc, storageConfig.Options.MaxSerializationRetries)
			k8spolicies_controller := resourcesctl.New(k8spolicies_repo, inventoryresources_repo, authorizer, eventingManager, "acm", log.With(logger, "subsystem", "k8spolicies_controller"), listenManager, waitForNotifCircuitBreaker, usecaseConfig)
			k8spolicies_service := k8spoliciessvc.NewKesselK8SPolicyServiceV1beta1(k8spolicies_controller)
			pb.RegisterKesselK8SPolicyServiceServer(server.GrpcServer, k8spolicies_service)
			pb.RegisterKesselK8SPolicyServiceHTTPServer(server.HttpServer, k8spolicies_service)

			// wire together relationships handling
			relationships_repo := relationshipsrepo.New(db)
			relationships_controller := relationshipsctl.New(relationships_repo, eventingManager, log.With(logger, "subsystem", "relationships_controller"), storageConfig.Options.DisablePersistence)
			relationships_service := relationshipssvc.NewKesselK8SPolicyIsPropagatedToK8SClusterServiceV1beta1(relationships_controller)
			rel.RegisterKesselK8SPolicyIsPropagatedToK8SClusterServiceServer(server.GrpcServer, relationships_service)
			rel.RegisterKesselK8SPolicyIsPropagatedToK8SClusterServiceHTTPServer(server.HttpServer, relationships_service)

			health_repo := healthrepo.New(db, authorizer, authzConfig)
			health_controller := healthctl.New(health_repo, log.With(logger, "subsystem", "health_controller"), storageConfig.Options.DisablePersistence)
			health_service := healthssvc.New(health_controller)
			hb.RegisterKesselInventoryHealthServiceServer(server.GrpcServer, health_service)
			hb.RegisterKesselInventoryHealthServiceHTTPServer(server.HttpServer, health_service)

			srvErrs := make(chan error)
			go func() {
				srvErrs <- server.Run(ctx)
			}()

			shutdown := shutdown(db, server, eventingManager, &inventoryConsumer, log.NewHelper(logger))

			if !storageOptions.DisablePersistence && consumerOptions.Enabled {
				go func() {
					retries := 0
					for consumerOptions.RetryOptions.ConsumerMaxRetries == -1 || retries < consumerOptions.RetryOptions.ConsumerMaxRetries {
						// If the consumer cannot process a message, the consumer loop is restarted
						// This is to ensure we re-read the message and prevent it being dropped and moving to next message.
						// To re-read the current message, we have to recreate the consumer connection so that the earliest offset is used
						inventoryConsumer, err = consumer.New(consumerConfig, db, authzConfig, authorizer, notifier, log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer")))
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

	return cmd
}

func shutdown(db *gorm.DB, srv *server.Server, em eventingapi.Manager, cm *consumer.InventoryConsumer, logger *log.Helper) func(reason interface{}) {
	return func(reason interface{}) {
		log.Info(fmt.Sprintf("Server Shutdown: %s", reason))

		timeout := srv.HttpServer.ReadTimeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down API: %v", err))
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
