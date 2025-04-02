package consumer

import (
	"fmt"

	"github.com/spf13/pflag"
)

type Options struct {
	Enabled                 bool     `mapstructure:"enabled"`
	BootstrapServers        string   `mapstructure:"bootstrap-servers"`
	ConsumerGroupID         string   `mapstructure:"consumer-group-id"`
	Topic                   string   `mapstructure:"topic"`
	ReadAfterWriteEnabled   bool     `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist []string `mapstructure:"read-after-write-allowlist"`
	SessionTimeout          string   `mapstructure:"session-timeout"`
	HeartbeatInterval       string   `mapstructure:"heartbeat-interval"`
	MaxPollInterval         string   `mapstructure:"max-poll-interval"`
	EnableAutoCommit        string   `mapstructure:"enable-auto-commit"`
	AutoOffsetReset         string   `mapstructure:"auto-offset-reset"`
	StatisticsInterval      string   `mapstructure:"statistics-interval-ms"`
	Debug                   string   `mapstructure:"debug"`
}

func NewOptions() *Options {
	return &Options{
		Enabled:            true,
		ConsumerGroupID:    "inventory-consumer",
		Topic:              "outbox.event.kessel.tuples",
		SessionTimeout:     "45000",
		HeartbeatInterval:  "3000",
		MaxPollInterval:    "300000",
		EnableAutoCommit:   "false",
		AutoOffsetReset:    "earliest",
		StatisticsInterval: "60000",
		Debug:              "",
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.Enabled, prefix+"enabled", o.Enabled, "Toggle for enabling or disabling the consumer (default: true)")
	fs.StringVar(&o.BootstrapServers, prefix+"bootstrap-servers", o.BootstrapServers, "sets the bootstrap server address and port for Kafka")
	fs.StringVar(&o.ConsumerGroupID, prefix+"consumer-groupd-id", o.ConsumerGroupID, "sets the Kafka consumer group name (default: inventory-consumer)")
	fs.StringVar(&o.Topic, prefix+"topic", o.Topic, "Kafka topic to monitor for events")
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow (default: true)")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled (default: [])")
	fs.StringVar(&o.SessionTimeout, prefix+"session-timeout", o.SessionTimeout, "time a consumer can live without sending heartbeat (default: 45000ms)")
	fs.StringVar(&o.HeartbeatInterval, prefix+"heartbeat-interval", o.HeartbeatInterval, "interval between heartbeats sent to Kafka (default: 3000ms, must be lower then session-timeout)")
	fs.StringVar(&o.MaxPollInterval, prefix+"max-poll", o.MaxPollInterval, "length of time consumer can go without polling before considered dead (default: 300000ms)")
	fs.StringVar(&o.EnableAutoCommit, prefix+"enable-auto-commit", o.EnableAutoCommit, "enables auto commit on consumer when messages are consumed (default: false)")
	fs.StringVar(&o.AutoOffsetReset, prefix+"auto-offset-reset", o.AutoOffsetReset, "action to take when there is no initial offset in offset store (default: earliest)")
	fs.StringVar(&o.StatisticsInterval, prefix+"statistics-interval", o.StatisticsInterval, "librdkafka statistics emit interval (default: 60000ms)")

	o.RetryOptions.AddFlags(fs, prefix+"retry-options")
}

func (o *Options) Validate() []error {
	var errs []error

	if len(o.BootstrapServers) == 0 && o.Enabled {
		errs = append(errs, fmt.Errorf("bootstrap servers can not be empty"))
	}
	return errs
}

func (o *Options) Complete() []error {
	return nil
}
