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
		dsnBuilder.WriteString(fmt.Sprintf("host=%s ", c.Host))
	}

	if c.Port != "" {
		dsnBuilder.WriteString(fmt.Sprintf("port=%s ", c.Port))
	}

	if c.DbName != "" {
		dsnBuilder.WriteString(fmt.Sprintf("dbname=%s ", c.DbName))
	}

	if c.User != "" {
		dsnBuilder.WriteString(fmt.Sprintf("user=%s ", c.User))
	}

	if c.Password != "" {
		dsnBuilder.WriteString(fmt.Sprintf("password=%s ", c.Password))
	}

	if c.SSLMode != "" {
		dsnBuilder.WriteString(fmt.Sprintf("sslmode=%s ", c.SSLMode))
	}

	if c.SSLRootCert != "" {
		dsnBuilder.WriteString(fmt.Sprintf("sslrootcert=%s ", c.SSLRootCert))
	}

	for k, v := range c.Extra {
		dsnBuilder.WriteString(fmt.Sprintf("%s=%s ", k, v))
	}

	dsn := strings.TrimSpace(dsnBuilder.String())

	return CompletedConfig{&completedConfig{DSN: dsn}}
}

func NewCompleteConfig(c *Config) CompletedConfig {
    return c.Complete()
}

