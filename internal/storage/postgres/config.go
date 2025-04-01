package postgres

import (
	"fmt"
	"strings"
)

type Config struct {
	*Options
	DSN string
}

type completedConfig struct {
	DSN string
}

// CompletedConfig can be constructed only from Config.Complete
type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		Options: o,
	}
}

func (c *Config) Complete() CompletedConfig {
	if c.DSN != "" {
		return CompletedConfig{&completedConfig{DSN: c.DSN}}
	}

	dsnBuilder := new(strings.Builder)
	if c.Host != "" {
		fmt.Fprintf(dsnBuilder, "host=%s ", c.Host)
	}

	if c.Port != "" {
		fmt.Fprintf(dsnBuilder, "port=%s ", c.Port)
	}

	if c.DbName != "" {
		fmt.Fprintf(dsnBuilder, "dbname=%s ", c.DbName)
	}

	if c.User != "" {
		fmt.Fprintf(dsnBuilder, "user=%s ", c.User)
	}

	if c.Password != "" {
		fmt.Fprintf(dsnBuilder, "password=%s ", c.Password)
	}

	if c.SSLMode != "" {
		fmt.Fprintf(dsnBuilder, "sslmode=%s ", c.SSLMode)
	}

	if c.SSLRootCert != "" {
		fmt.Fprintf(dsnBuilder, "sslrootcert=%s ", c.SSLRootCert)
	}

	dsn := strings.TrimSpace(dsnBuilder.String())

	return CompletedConfig{&completedConfig{DSN: dsn}}
}
