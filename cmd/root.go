package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"strconv"

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
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
)

// go build -ldflags "-X cmd.Version=x.y.z"
var (
	Version = "0.1.0"
	Name    = "inventory-api"
	cfgFile string

	logger *log.Helper

	baseLogger log.Logger

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

	if clowder.IsClowderEnabled() {
		options.Storage.Postgres.Host = clowder.LoadedConfig.Database.Hostname
		options.Storage.Postgres.Port = strconv.Itoa(clowder.LoadedConfig.Database.Port)
		options.Storage.Postgres.User = clowder.LoadedConfig.Database.Username
		options.Storage.Postgres.Password = clowder.LoadedConfig.Database.Password
		options.Storage.Postgres.DbName = clowder.LoadedConfig.Database.Name
	}
  
	// TODO: Find a cleaner approach than explicitly calling initConfig
	// Here we are calling the initConfig to ensure that the log level can be pulled from the inventory configuration file
	initConfig()
	logLevel := getLogLevel()
	logger, baseLogger = initLogger(logLevel)

	migrateCmd := migrate.NewCommand(options.Storage, baseLogger)
	rootCmd.AddCommand(migrateCmd)
	err = viper.BindPFlags(migrateCmd.Flags())
	if err != nil {
		panic(err)
	}
	serveCmd := serve.NewCommand(options.Server, options.Storage, options.Authn, options.Authz, options.Eventing, baseLogger)
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
		log.Infof("Using config file: %s", viper.ConfigFileUsed())
	}

	// put the values into the options struct.
	if err := viper.Unmarshal(&options); err != nil {
		panic(err)
	}
}

func getLogLevel() string {

	logLevel := viper.GetString("log.level")
	fmt.Printf("Log Level is set to: %s\n", logLevel)
	return logLevel
}

// initLogger initializes the logger based on the provided log level
func initLogger(logLevel string) (*log.Helper, log.Logger) {
	// Convert logLevel string to log.Level type
	var level log.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = log.LevelDebug
	case "info":
		level = log.LevelInfo
	case "warn":
		level = log.LevelWarn
	case "error":
		level = log.LevelError
	case "fatal":
		level = log.LevelFatal
	default:
		fmt.Printf("Invalid log level '%s' provided. Defaulting to 'info' level.\n", logLevel)
		level = log.LevelInfo
	}

	rootLogger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	filteredLogger := log.NewFilter(rootLogger, log.FilterLevel(level))
	helperLogger := log.NewHelper(filteredLogger)

	return helperLogger, filteredLogger
}
