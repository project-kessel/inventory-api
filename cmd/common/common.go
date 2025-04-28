package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/spf13/viper"
)

type LoggerOptions struct {
	ServiceName    string
	ServiceVersion string
}

func GetLogLevel() string {
	logLevel := viper.GetString("log.level")
	fmt.Printf("Log Level is set to: %s\n", logLevel)
	return logLevel
}

// InitLogger initializes the logger based on the provided log level
func InitLogger(logLevel string, options LoggerOptions) (*log.Helper, log.Logger) {
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
		"service.name", options.ServiceName,
		"service.version", options.ServiceVersion,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	filteredLogger := log.NewFilter(rootLogger, log.FilterLevel(level))
	helperLogger := log.NewHelper(filteredLogger)

	return helperLogger, filteredLogger
}
