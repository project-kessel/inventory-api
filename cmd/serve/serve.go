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

	"github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta1"

	hostsrepo "github.com/project-kessel/inventory-api/internal/data/hosts"
	k8sclustersrepo "github.com/project-kessel/inventory-api/internal/data/k8sclusters"
	policiesrepo "github.com/project-kessel/inventory-api/internal/data/policies"
	relationshipsrepo "github.com/project-kessel/inventory-api/internal/data/relationships"

	hostsctl "github.com/project-kessel/inventory-api/internal/biz/hosts"
	k8sclustersctl "github.com/project-kessel/inventory-api/internal/biz/k8sclusters"
	policiesctl "github.com/project-kessel/inventory-api/internal/biz/policies"
	relationshipsctl "github.com/project-kessel/inventory-api/internal/biz/relationships"

	hostssvc "github.com/project-kessel/inventory-api/internal/service/hosts"
	k8sclusterssvc "github.com/project-kessel/inventory-api/internal/service/k8sclusters"
	policiessvc "github.com/project-kessel/inventory-api/internal/service/policies"
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

			// construct the authn
			authenticator, err := authn.New(authnConfig)
			if err != nil {
				return err
			}

			// construct the authz
			authorizer, err := authz.New(ctx, authzConfig, log.NewHelper(log.With(logger, "subsystem", "authz")))
			if err != nil {
				return err
			}

			// construct eventing
			eventingManager, err := eventing.New(eventingConfig, log.NewHelper(log.With(logger, "subsystem", "eventing")))
			if err != nil {
				return err
			}

			// construct the servers
			server := server.New(serverConfig, middleware.Authentication(authenticator), logger)

			// wire together hosts handling
			hosts_repo := hostsrepo.New(db, authorizer, eventingManager, log.NewHelper(log.With(logger, "subsystem", "hosts_repo")))
			hosts_controller := hostsctl.New(hosts_repo, log.With(logger, "subsystem", "hosts_controller"))
			hosts_service := hostssvc.New(hosts_controller)
			v1beta1.RegisterHostsServiceServer(server.GrpcServer, hosts_service)
			v1beta1.RegisterHostsServiceHTTPServer(server.HttpServer, hosts_service)

			// wire together k8sclusters handling
			k8sclusters_repo := k8sclustersrepo.New(db, log.NewHelper(log.With(logger, "subsystem", "k8sclusters_repo")))
			k8sclusters_controller := k8sclustersctl.New(k8sclusters_repo, log.With(logger, "subsystem", "k8sclusters_controller"))
			k8sclusters_service := k8sclusterssvc.New(k8sclusters_controller)
			v1beta1.RegisterK8SClustersServiceServer(server.GrpcServer, k8sclusters_service)
			v1beta1.RegisterK8SClustersServiceHTTPServer(server.HttpServer, k8sclusters_service)

			// wire together policies handling
			policies_repo := policiesrepo.New(db, log.NewHelper(log.With(logger, "subsystem", "policies_repo")))
			policies_controller := policiesctl.New(policies_repo, log.With(logger, "subsystem", "policies_controller"))
			policies_service := policiessvc.New(policies_controller)
			v1beta1.RegisterPoliciesServiceServer(server.GrpcServer, policies_service)
			v1beta1.RegisterPoliciesServiceHTTPServer(server.HttpServer, policies_service)

			// wire together relationships handling
			relationships_repo := relationshipsrepo.New(db, log.NewHelper(log.With(logger, "subsystem", "relationships_repo")))
			relationships_controller := relationshipsctl.New(relationships_repo, log.With(logger, "subsystem", "relationships_controller"))
			relationships_service := relationshipssvc.New(relationships_controller)
			v1beta1.RegisterRelationshipsServiceServer(server.GrpcServer, relationships_service)
			v1beta1.RegisterRelationshipsServiceHTTPServer(server.HttpServer, relationships_service)

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
	storageOptions.AddFlags(cmd.Flags(), "storage")
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
