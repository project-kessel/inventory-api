package psk

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	PreSharedKeyFile string
	Keys             IdentityMap
}

type completedConfig struct {
	PreSharedKeyFile string
	Keys             IdentityMap
}

type CompletedConfig struct {
	*completedConfig
}

func NewConfig(o *Options) *Config {
	return &Config{
		PreSharedKeyFile: o.PreSharedKeyFile,
	}
}

func (c *Config) Complete() (CompletedConfig, error) {
	if len(c.Keys) == 0 {
		if err := c.loadPreSharedKeys(); err != nil {
			return CompletedConfig{}, err
		}
	}

	return CompletedConfig{&completedConfig{
		PreSharedKeyFile: c.PreSharedKeyFile,
		Keys:             c.Keys,
	}}, nil
}

func (c *Config) loadPreSharedKeys() error {
	if len(c.PreSharedKeyFile) > 0 {
		// TODO: fsnotify to reload the keys when the PSK file changes.
		if file, err := os.Open(c.PreSharedKeyFile); err == nil {
			defer func() {
				if err := file.Close(); err != nil {
					fmt.Printf("failed to close consumer: %v", err)
				}
			}()
			data, err := io.ReadAll(file)
			if err == nil {
				if err := yaml.Unmarshal(data, &c.Keys); err != nil {
					return fmt.Errorf("failed to unmarshal preshared key: %w", err)
				}
			} else {
				return fmt.Errorf("failed to read preshared key file: %w", err)
			}
		} else {
			return fmt.Errorf("error opening preshared key file: %s [%s]", c.PreSharedKeyFile, err.Error())
		}
	}
	return nil
}
