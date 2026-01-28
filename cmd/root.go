package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/cmd/jobs"
	"github.com/project-kessel/inventory-api/cmd/migrate"
	"github.com/project-kessel/inventory-api/cmd/schema"
	"github.com/project-kessel/inventory-api/cmd/serve"
	"github.com/project-kessel/inventory-api/internal/provider"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// go build -ldflags "-X cmd.Version=x.y.z"
var (
	// Version is the application version, can be set at build time using ldflags.
	Version = "0.1.0"
	// Name is the application name used for configuration and commands.
	Name    = "inventory-api"
	cfgFile string

	logger *log.Helper

	rootCmd = &cobra.Command{
		Use:     Name,
		Version: Version,
		Short:   "A simple common inventory system",
	}

	options = provider.NewOptions()
)

// Execute is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// Initialize sets up configuration and logging for the application.
func Initialize() {
	initConfig()

	// for troubleshoot, when set to debug, configuration info is logged in more detail to stdout
	logLevel := common.GetLogLevel()
	logger, _ = common.InitLogger(logLevel, common.LoggerOptions{
		ServiceName:    Name,
		ServiceVersion: Version,
	})
	if logLevel == "debug" {
		LogConfigurationInfo(options)
	}
}

// init adds all child commands to the root command and sets flags appropriately.
func init() {
	// initializers are run as part of Command.PreRun
	cobra.OnInitialize(Initialize)

	configHelp := fmt.Sprintf("config file (default is $PWD/.%s.yaml)", Name)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", configHelp)

	// options.Storage is used in both commands
	// viper/cobra do not handle well when the same key is used in multiple sub-commands
	// and the last one takes precedence
	// Set them as persistent flags in the main command
	// Another solution might involve into creating our own set of flags to prevent having two separate objects for
	// the same flag.
	options.Storage.AddFlags(rootCmd.PersistentFlags(), "storage")

	err := viper.BindPFlags(rootCmd.PersistentFlags())
	if err != nil {
		panic(err)
	}

	if clowder.IsClowderEnabled() {
		err := options.InjectClowdAppConfig(clowder.LoadedConfig)
		if err != nil {
			panic(err)
		}
	}
	loggerOptions := common.LoggerOptions{
		ServiceName:    Name,
		ServiceVersion: Version,
	}

	migrateCmd := migrate.NewCommand(options.Storage, loggerOptions)
	rootCmd.AddCommand(migrateCmd)
	err = viper.BindPFlags(migrateCmd.Flags())
	if err != nil {
		panic(err)
	}

	serveCmd := serve.NewCommand(options, loggerOptions)
	rootCmd.AddCommand(serveCmd)
	err = viper.BindPFlags(serveCmd.Flags())
	if err != nil {
		panic(err)
	}

	schemaCmd := schema.NewCommand(loggerOptions)
	rootCmd.AddCommand(schemaCmd)
	err = viper.BindPFlags(schemaCmd.Flags())
	if err != nil {
		panic(err)
	}

	runJobCmd := jobs.NewRunJobCommand(options.Storage, loggerOptions)
	rootCmd.AddCommand(runJobCmd)
	err = viper.BindPFlags(runJobCmd.Flags())
	if err != nil {
		panic(err)
	}
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		if configFilePath, exists := os.LookupEnv("INVENTORY_API_CONFIG"); exists {
			absPath, err := filepath.Abs(configFilePath)
			if err != nil {
				log.Fatalf("Failed to resolve absolute path for config file: %v", err)
			}
			// Set the config file path
			viper.SetConfigFile(absPath)
			if err := viper.ReadInConfig(); err != nil {
				log.Fatalf("Error reading INVENTORY_API_CONFIG file, %s", err)
			}
		} else {
			home, err := os.UserHomeDir()
			cobra.CheckErr(err)

			viper.AddConfigPath(".")
			viper.AddConfigPath(home)
			viper.SetConfigType("yaml")

			viper.SetConfigName("." + Name)
		}
	}

	viper.SetEnvPrefix(strings.ReplaceAll(Name, "-", "_"))
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	} else {
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	// put the values into the options struct.
	if err := viper.Unmarshal(&options); err != nil {
		panic(err)
	}
}

// LogConfigurationInfo outputs connection details to logs when in debug for testing (no secret data is output)
func LogConfigurationInfo(options *provider.Options) {
	log.Debugf("Server Configuration: Public URL: %s, HTTP Listener: %s, GRPC Listener: %s",
		options.Server.PublicUrl,
		options.Server.Http.Addr,
		options.Server.Grpc.Addr)

	// Log authn configuration if OIDC is configured
	if options.Authn.Authenticator != nil {
		for _, entry := range options.Authn.Authenticator.Chain {
			if entry.Type == "oidc" {
				if authServerURL, ok := entry.Config[provider.OIDCConfigKeyAuthServerURL].(string); ok && authServerURL != "" {
					clientID := ""
					if cid, ok := entry.Config[provider.OIDCConfigKeyClientID].(string); ok {
						clientID = cid
					}
					log.Debugf("Authn Configuration: URL: %s, ClientID: %s", authServerURL, clientID)
				}
				break
			}
		}
	}

	if options.Storage.Database == provider.DatabasePostgres {
		log.Debugf("Storage Configuration: Host: %s, DB: %s, Port: %s, SSLMode: %s, RDSCACert: %s",
			options.Storage.Postgres.Host,
			options.Storage.Postgres.DbName,
			options.Storage.Postgres.Port,
			options.Storage.Postgres.SSLMode,
			options.Storage.Postgres.SSLRootCert,
		)
	}

	if options.Authz.Authz == provider.AuthzKessel {
		log.Debugf("Authz Configuration: URL: %s, Insecure?: %t, OIDC?: %t",
			options.Authz.Kessel.URL,
			options.Authz.Kessel.Insecure,
			options.Authz.Kessel.EnableOidcAuth,
		)
	}

	log.Debugf("Consumer Configuration: Bootstrap Server: %s, Topic: %s, Consumer Max Retries: %d, Operation Max Retries: %d, Backoff Factor: %d, Max Backoff Seconds: %d",
		options.Consumer.BootstrapServers,
		options.Consumer.Topic,
		options.Consumer.Retry.ConsumerMaxRetries,
		options.Consumer.Retry.OperationMaxRetries,
		options.Consumer.Retry.BackoffFactor,
		options.Consumer.Retry.MaxBackoffSeconds,
	)

	log.Debugf("Consumer Auth Settings: Enabled: %v, Security Protocol: %s, Mechanism: %s, Username: %s",
		options.Consumer.Auth.Enabled,
		options.Consumer.Auth.SecurityProtocol,
		options.Consumer.Auth.SASLMechanism,
		options.Consumer.Auth.SASLUsername)

	log.Debugf("Consistency Configuration: Read-After-Write Enabled: %t, Read-After-Write Allowlist: %+s",
		options.Consistency.ReadAfterWriteEnabled,
		options.Consistency.ReadAfterWriteAllowlist,
	)
}
