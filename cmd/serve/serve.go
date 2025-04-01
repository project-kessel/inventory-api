package serve

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/project-kessel/inventory-api/internal/consumer"

	"github.com/project-kessel/inventory-api/cmd/common"
	relationshipsctl "github.com/project-kessel/inventory-api/internal/biz/relationships"
	resourcesctl "github.com/project-kessel/inventory-api/internal/biz/resources"
	inventoryResourcesRepo "github.com/project-kessel/inventory-api/internal/data/inventoryresources"
	relationshipsrepo "github.com/project-kessel/inventory-api/internal/data/relationships"
	resourcerepo "github.com/project-kessel/inventory-api/internal/data/resources"
	"github.com/project-kessel/inventory-api/internal/pubsub"
	authzsvc "github.com/project-kessel/inventory-api/internal/service/authz"
	relationshipssvc "github.com/project-kessel/inventory-api/internal/service/relationships/k8spolicy"
	hostssvc "github.com/project-kessel/inventory-api/internal/service/resources/hosts"
	k8sclusterssvc "github.com/project-kessel/inventory-api/internal/service/resources/k8sclusters"
	k8spoliciessvc "github.com/project-kessel/inventory-api/internal/service/resources/k8spolicies"
	notifssvc "github.com/project-kessel/inventory-api/internal/service/resources/notificationsintegrations"

	//v1beta2
	resourcesvc "github.com/project-kessel/inventory-api/internal/service/resources"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/go-kratos/kratos/v2/log"

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
	authzv1beta2 "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2/authz"
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
			// construct pubsub
			pubSubLogger := log.NewHelper(log.With(logger, "subsystem", "pubsub"))
			pgxPool, err := storage.NewPgx(storageConfig, pubSubLogger)
			if err != nil {
				// TODO should not completely fail for sqllite
				return err
			}

			// setup the driver listener
			listener := pubsub.NewDriver(pgxPool)
			err = listener.Listen(ctx, "consumer-notifications")
			if err != nil {
				return fmt.Errorf("error setting up listener: %v", err)
			}

			// setup the notifier
			listenManager := pubsub.NewListenManager(pubSubLogger, listener)
			go listenManager.Run(ctx)

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

			if !storageOptions.DisablePersistence && consumerOptions.Enabled {
				inventoryConsumer, err = consumer.New(consumerConfig, db, authzConfig, authorizer, log.NewHelper(log.With(logger, "subsystem", "inventoryConsumer")))
				if err != nil {
					return err
				}
			}
			// construct servers
			server, err := server.New(serverConfig, middleware.Authentication(authenticator), logger)
			if err != nil {
				return err
			}

			inventoryresources_repo := inventoryResourcesRepo.New(db)

			//v1beta2
			// wire together resource handling
			resource_repo := resourcerepo.New(db)
			resource_controller := resourcesctl.New(resource_repo, inventoryresources_repo, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), storageConfig.Options.DisablePersistence, listenManager)
			resource_service := resourcesvc.New(resource_controller)
			pbv1beta2.RegisterKesselResourceServiceServer(server.GrpcServer, resource_service)
			pbv1beta2.RegisterKesselResourceServiceHTTPServer(server.HttpServer, resource_service)

			// wire together authz handling
			authz_repov2 := resourcerepo.New(db)
			authz_controllerv2 := resourcesctl.New(authz_repov2, inventoryresources_repo, authorizer, eventingManager, "authz", log.With(logger, "subsystem", "authz_controller"), storageConfig.Options.DisablePersistence, listenManager)
			authz_servicev2 := authzsvc.NewV1beta2(authz_controllerv2)
			authzv1beta2.RegisterKesselCheckServiceServer(server.GrpcServer, authz_servicev2)
			authzv1beta2.RegisterKesselCheckServiceHTTPServer(server.HttpServer, authz_servicev2)

			//v1beta1
			// wire together notificationsintegrations handling
			notifs_repo := resourcerepo.New(db)
			notifs_controller := resourcesctl.New(notifs_repo, inventoryresources_repo, authorizer, eventingManager, "notifications", log.With(logger, "subsystem", "notificationsintegrations_controller"), storageConfig.Options.DisablePersistence, listenManager)
			notifs_service := notifssvc.New(notifs_controller)
			pb.RegisterKesselNotificationsIntegrationServiceServer(server.GrpcServer, notifs_service)
			pb.RegisterKesselNotificationsIntegrationServiceHTTPServer(server.HttpServer, notifs_service)

			// wire together authz handling
			authz_repo := resourcerepo.New(db)
			authz_controller := resourcesctl.New(authz_repo, inventoryresources_repo, authorizer, eventingManager, "authz", log.With(logger, "subsystem", "authz_controller"), storageConfig.Options.DisablePersistence, listenManager)
			authz_service := authzsvc.New(authz_controller)
			authzv1beta1.RegisterKesselCheckServiceServer(server.GrpcServer, authz_service)
			authzv1beta1.RegisterKesselCheckServiceHTTPServer(server.HttpServer, authz_service)

			// wire together hosts handling
			hosts_repo := resourcerepo.New(db)
			hosts_controller := resourcesctl.New(hosts_repo, inventoryresources_repo, authorizer, eventingManager, "hbi", log.With(logger, "subsystem", "hosts_controller"), storageConfig.Options.DisablePersistence, listenManager)
			hosts_service := hostssvc.New(hosts_controller)
			pb.RegisterKesselRhelHostServiceServer(server.GrpcServer, hosts_service)
			pb.RegisterKesselRhelHostServiceHTTPServer(server.HttpServer, hosts_service)

			// wire together k8sclusters handling
			k8sclusters_repo := resourcerepo.New(db)
			k8sclusters_controller := resourcesctl.New(k8sclusters_repo, inventoryresources_repo, authorizer, eventingManager, "acm", log.With(logger, "subsystem", "k8sclusters_controller"), storageConfig.Options.DisablePersistence, listenManager)
			k8sclusters_service := k8sclusterssvc.New(k8sclusters_controller)
			pb.RegisterKesselK8SClusterServiceServer(server.GrpcServer, k8sclusters_service)
			pb.RegisterKesselK8SClusterServiceHTTPServer(server.HttpServer, k8sclusters_service)

			// wire together k8spolicies handling
			k8spolicies_repo := resourcerepo.New(db)
			k8spolicies_controller := resourcesctl.New(k8spolicies_repo, inventoryresources_repo, authorizer, eventingManager, "acm", log.With(logger, "subsystem", "k8spolicies_controller"), storageConfig.Options.DisablePersistence, listenManager)
			k8spolicies_service := k8spoliciessvc.New(k8spolicies_controller)
			pb.RegisterKesselK8SPolicyServiceServer(server.GrpcServer, k8spolicies_service)
			pb.RegisterKesselK8SPolicyServiceHTTPServer(server.HttpServer, k8spolicies_service)

			// wire together relationships handling
			relationships_repo := relationshipsrepo.New(db)
			relationships_controller := relationshipsctl.New(relationships_repo, eventingManager, log.With(logger, "subsystem", "relationships_controller"), storageConfig.Options.DisablePersistence)
			relationships_service := relationshipssvc.New(relationships_controller)
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

			if !storageOptions.DisablePersistence && consumerOptions.Enabled {
				go func() {
					err := inventoryConsumer.Consume()
					if err != nil {
						inventoryConsumer.Errors <- err
					}
				}()
			}

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			shutdown := shutdown(db, server, eventingManager, inventoryConsumer, log.NewHelper(logger))

			select {
			case err := <-srvErrs:
				shutdown(err)
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

	return cmd
}

func shutdown(db *gorm.DB, srv *server.Server, em eventingapi.Manager, cm consumer.InventoryConsumer, logger *log.Helper) func(reason interface{}) {
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

		if cm != (consumer.InventoryConsumer{}) {
			if !cm.Consumer.IsClosed() {
				if err := cm.Shutdown(); err != nil {
					logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Consumer: %v", err))
				}
			}
		}

		if sqlDB, err := db.DB(); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Storage: %v", err))
		} else {
			sqlDB.Close()
		}
	}
}
