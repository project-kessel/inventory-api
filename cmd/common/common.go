package common

import (
	"fmt"
	"os"
	"reflect"
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

func ToPointer[T any](v T) *T {
	return &v
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

// IsNil checks if the given interface is nil or not
func IsNil(i interface{}) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Ptr, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return reflect.ValueOf(i).IsNil()
	case reflect.Array:
		return reflect.ValueOf(i).IsZero()
	}
	return false
}
