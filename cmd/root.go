package cmd

import (
	"fmt"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/project-kessel/inventory-api/cmd/common"
	"github.com/project-kessel/inventory-api/cmd/migrate"
	"github.com/project-kessel/inventory-api/cmd/serve"
	"github.com/project-kessel/inventory-api/internal/config"
	clowder "github.com/redhatinsights/app-common-go/pkg/api/v1"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

// go build -ldflags "-X cmd.Version=x.y.z"
var (
	Version = "0.1.0"
	Name    = "inventory-api"
	cfgFile string

	logger *log.Helper

	rootCmd = &cobra.Command{
		Use:     Name,
		Version: Version,
		Short:   "A simple common inventory system",
	}

	options = config.NewOptionsConfig()
)

// Execute is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}
}

func Initialize() {
	initConfig()

	// for troubleshoot, when set to debug, configuration info is logged in more detail to stdout
	logLevel := common.GetLogLevel()
	logger, _ = common.InitLogger(logLevel, common.LoggerOptions{
		ServiceName:    Name,
		ServiceVersion: Version,
	})
	if logLevel == "debug" {
		config.LogConfigurationInfo(options)
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
		options.InjectClowdAppConfig()
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
	serveCmd := serve.NewCommand(options.Server, options.Storage, options.Authn, options.Authz, options.Eventing, loggerOptions)
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
