package serve

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	rel "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/relationships"
	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1/resources"

	healthrepo "github.com/project-kessel/inventory-api/internal/data/health"
	hostsrepo "github.com/project-kessel/inventory-api/internal/data/hosts"
	k8sclustersrepo "github.com/project-kessel/inventory-api/internal/data/k8sclusters"
	k8spoliciesrepo "github.com/project-kessel/inventory-api/internal/data/k8spolicies"
	notifsrepo "github.com/project-kessel/inventory-api/internal/data/notificationsintegrations"
	relationshipsrepo "github.com/project-kessel/inventory-api/internal/data/relationships"

	healthctl "github.com/project-kessel/inventory-api/internal/biz/health"
	hostsctl "github.com/project-kessel/inventory-api/internal/biz/hosts"
	k8sclustersctl "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	k8spoliciesctl "github.com/project-kessel/inventory-api/internal/biz/k8spolicies"
	notifsctl "github.com/project-kessel/inventory-api/internal/biz/notificationsintegrations"
	relationshipsctl "github.com/project-kessel/inventory-api/internal/biz/relationships"

	healthssvc "github.com/project-kessel/inventory-api/internal/service/health"
	hostssvc "github.com/project-kessel/inventory-api/internal/service/hosts"
	k8sclusterssvc "github.com/project-kessel/inventory-api/internal/service/k8sclusters"
	k8spoliciessvc "github.com/project-kessel/inventory-api/internal/service/k8spolicies"
	notifssvc "github.com/project-kessel/inventory-api/internal/service/notificationsintegrations"
	relationshipssvc "github.com/project-kessel/inventory-api/internal/service/relationships"
)

func NewCommand(
	serverOptions *server.Options,
	storageOptions *storage.Options,
	authnOptions *authn.Options,
	authzOptions *authz.Options,
	eventingOptions *eventing.Options,
	logger log.Logger,
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the inventory server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

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
			eventingManager, err := eventing.New(eventingConfig, serverConfig.Options.Id, log.NewHelper(log.With(logger, "subsystem", "eventing")))
			if err != nil {
				return err
			}

			// construct servers
			server, err := server.New(serverConfig, middleware.Authentication(authenticator), logger)
			if err != nil {
				return err
			}

			// wire together notificationsintegrations handling
			notifs_repo := notifsrepo.New(db, authorizer, eventingManager)
			notifs_controller := notifsctl.New(notifs_repo, log.With(logger, "subsystem", "notificationsintegrations_controller"))
			notifs_service := notifssvc.New(notifs_controller)
			pb.RegisterKesselNotificationsIntegrationServiceServer(server.GrpcServer, notifs_service)
			pb.RegisterKesselNotificationsIntegrationServiceHTTPServer(server.HttpServer, notifs_service)

			// wire together hosts handling
			hosts_repo := hostsrepo.New(db, authorizer, eventingManager)
			hosts_controller := hostsctl.New(hosts_repo, log.With(logger, "subsystem", "hosts_controller"))
			hosts_service := hostssvc.New(hosts_controller)
			pb.RegisterKesselRhelHostServiceServer(server.GrpcServer, hosts_service)
			pb.RegisterKesselRhelHostServiceHTTPServer(server.HttpServer, hosts_service)

			// wire together k8sclusters handling
			k8sclusters_repo := k8sclustersrepo.New(db, authorizer, eventingManager)
			k8sclusters_controller := k8sclustersctl.New(k8sclusters_repo, log.With(logger, "subsystem", "k8sclusters_controller"))
			k8sclusters_service := k8sclusterssvc.New(k8sclusters_controller)
			pb.RegisterKesselK8SClusterServiceServer(server.GrpcServer, k8sclusters_service)
			pb.RegisterKesselK8SClusterServiceHTTPServer(server.HttpServer, k8sclusters_service)

			// wire together k8spolicies handling
			k8spolicies_repo := k8spoliciesrepo.New(db, authorizer, eventingManager)
			k8spolicies_controller := k8spoliciesctl.New(k8spolicies_repo, log.With(logger, "subsystem", "k8spolicies_controller"))
			k8spolicies_service := k8spoliciessvc.New(k8spolicies_controller)
			pb.RegisterKesselK8SPolicyServiceServer(server.GrpcServer, k8spolicies_service)
			pb.RegisterKesselK8SPolicyServiceHTTPServer(server.HttpServer, k8spolicies_service)

			// wire together relationships handling
			relationships_repo := relationshipsrepo.New(db)
			relationships_controller := relationshipsctl.New(relationships_repo, log.With(logger, "subsystem", "relationships_controller"))
			relationships_service := relationshipssvc.New(relationships_controller)
			rel.RegisterKesselK8SPolicyIsPropagatedToK8SClusterServiceServer(server.GrpcServer, relationships_service)
			rel.RegisterKesselK8SPolicyIsPropagatedToK8SClusterServiceHTTPServer(server.HttpServer, relationships_service)

			health_repo := healthrepo.New(db, authorizer, authzConfig)
			health_controller := healthctl.New(health_repo, log.With(logger, "subsystem", "health_controller"))
			health_service := healthssvc.New(health_controller)
			hb.RegisterKesselInventoryHealthServiceServer(server.GrpcServer, health_service)
			hb.RegisterKesselInventoryHealthServiceHTTPServer(server.HttpServer, health_service)

			srvErrs := make(chan error)
			go func() {
				srvErrs <- server.Run(ctx)
			}()

			quit := make(chan os.Signal, 1)
			signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

			shutdown := shutdown(db, server, eventingManager, log.NewHelper(logger))

			select {
			case err := <-srvErrs:
				shutdown(err)
			case sig := <-quit:
				shutdown(sig)
			case emErr := <-eventingManager.Errs():
				shutdown(emErr)
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

func shutdown(db *gorm.DB, srv *server.Server, em eventingapi.Manager, logger *log.Helper) func(reason interface{}) {
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

		if sqlDB, err := db.DB(); err != nil {
			logger.Error(fmt.Sprintf("Error Gracefully Shutting Down Storage: %v", err))
		} else {
			sqlDB.Close()
		}
	}
}
