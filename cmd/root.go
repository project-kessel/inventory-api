package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"

	"github.com/project-kessel/inventory-api/cmd/migrate"
	"github.com/project-kessel/inventory-api/cmd/serve"

	"github.com/project-kessel/inventory-api/internal/authn"
	"github.com/project-kessel/inventory-api/internal/authz"
	"github.com/project-kessel/inventory-api/internal/eventing"
	"github.com/project-kessel/inventory-api/internal/server"
	"github.com/project-kessel/inventory-api/internal/storage"
)

// go build -ldflags "-X cmd.Version=x.y.z"
var (
	Version = "0.1.0"
	Name    = "inventory-api"
	cfgFile string

	rootLog = log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	logger = log.NewHelper(log.NewFilter(rootLog, log.FilterLevel(log.LevelInfo)))

	rootCmd = &cobra.Command{
		Use:     Name,
		Version: Version,
		Short:   "A simple common inventory system",
	}

	options = struct {
		Authn    *authn.Options    `mapstructure:"authn"`
		Authz    *authz.Options    `mapstructure:"authz"`
		Storage  *storage.Options  `mapstructure:"storage"`
		Eventing *eventing.Options `mapstructure:"eventing"`
		Server   *server.Options   `mapstructure:"server"`
	}{
		authn.NewOptions(),
		authz.NewOptions(),
		storage.NewOptions(),
		eventing.NewOptions(),
		server.NewOptions(),
	}
)

// Execute is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

// init adds all child commands to the root command and sets flags appropriately.
func init() {

	// enable the randomness pool to improve uuid generation performance
	uuid.EnableRandPool()

	// initializers are run as part of Command.PreRun
	cobra.OnInitialize(initConfig)

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
	migrateCmd := migrate.NewCommand(options.Storage, log.With(rootLog, "group", "storage"))
	rootCmd.AddCommand(migrateCmd)
	err = viper.BindPFlags(migrateCmd.Flags())
	if err != nil {
		panic(err)
	}
	serveCmd := serve.NewCommand(options.Server, options.Storage, options.Authn, options.Authz, options.Eventing, log.With(rootLog, "group", "server"))
	rootCmd.AddCommand(serveCmd)
	err = viper.BindPFlags(serveCmd.Flags())
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

	viper.SetEnvPrefix(Name)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		panic(err)
	} else {
		logger.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	// put the values into the options struct.
	if err := viper.Unmarshal(&options); err != nil {
		panic(err)
	}
}
