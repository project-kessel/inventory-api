package consistency

import (
	"fmt"
	"strings"

	"github.com/spf13/pflag"

	pb "github.com/project-kessel/inventory-api/api/kessel/inventory/v1beta2"
)

// ConsistencyMode represents the available consistency modes for Check operations
type ConsistencyMode string

const (
	// MinimizeLatency selects the fastest snapshot available (default)
	MinimizeLatency ConsistencyMode = "minimizeLatency"
	// AtLeastAsFresh ensures data is at least as fresh as the provided token
	AtLeastAsFresh ConsistencyMode = "atLeastAsFresh"
)

// CheckOptions contains configuration options for Check operations
type CheckOptions struct {
	DefaultConsistencyMode ConsistencyMode `mapstructure:"defaultConsistencyMode"`
}

// NewCheckOptions creates CheckOptions with default values
func NewCheckOptions() *CheckOptions {
	return &CheckOptions{
		DefaultConsistencyMode: MinimizeLatency,
	}
}

type Options struct {
	ReadAfterWriteEnabled   bool          `mapstructure:"read-after-write-enabled"`
	ReadAfterWriteAllowlist []string      `mapstructure:"read-after-write-allowlist"`
	Check                   *CheckOptions `mapstructure:"check"`
}

func NewOptions() *Options {
	return &Options{
		ReadAfterWriteEnabled:   true,
		ReadAfterWriteAllowlist: []string{},
		Check:                   NewCheckOptions(),
	}
}

func (o *Options) AddFlags(fs *pflag.FlagSet, prefix string) {
	if prefix != "" {
		prefix = prefix + "."
	}
	fs.BoolVar(&o.ReadAfterWriteEnabled, prefix+"read-after-write-enabled", o.ReadAfterWriteEnabled, "Toggle for enabling or disabling the read after write consistency workflow (default: true)")
	fs.StringArrayVar(&o.ReadAfterWriteAllowlist, prefix+"read-after-write-allowlist", o.ReadAfterWriteAllowlist, "List of services that require all requests to be read-after-write enabled (default: [])")

	// Check options
	checkPrefix := prefix + "check."
	fs.StringVar((*string)(&o.Check.DefaultConsistencyMode), checkPrefix+"defaultConsistencyMode", string(o.Check.DefaultConsistencyMode),
		"Default consistency mode for Check operations: 'minimizeLatency' (default) or 'atLeastAsFresh'")
}

func (o *Options) Validate() []error {
	var errs []error

	// Validate consistency mode
	if o.Check != nil {
		mode := o.Check.DefaultConsistencyMode
		if mode != MinimizeLatency && mode != AtLeastAsFresh {
			errs = append(errs, fmt.Errorf("invalid consistency mode '%s': must be 'minimizeLatency' or 'atLeastAsFresh'", mode))
		}
	}

	return errs
}

func (o *Options) Complete() []error {
	// Ensure Check options exist
	if o.Check == nil {
		o.Check = NewCheckOptions()
	}
	return nil
}

// BuildConsistency creates a Consistency protobuf message from the mode and optional token.
// If mode is AtLeastAsFresh and token is empty, falls back to MinimizeLatency.
func BuildConsistency(mode ConsistencyMode, token string) *pb.Consistency {
	switch mode {
	case AtLeastAsFresh:
		if token != "" {
			return &pb.Consistency{
				Requirement: &pb.Consistency_AtLeastAsFresh{
					AtLeastAsFresh: &pb.ConsistencyToken{Token: token},
				},
			}
		}
		// Fall back to minimize latency if no token provided
		fallthrough
	case MinimizeLatency:
		fallthrough
	default:
		return &pb.Consistency{
			Requirement: &pb.Consistency_MinimizeLatency{MinimizeLatency: true},
		}
	}
}

// ParseConsistencyMode converts a string to ConsistencyMode, case-insensitive
func ParseConsistencyMode(s string) (ConsistencyMode, error) {
	switch strings.ToLower(s) {
	case "minimizelatency", "minimize_latency", "minimize-latency":
		return MinimizeLatency, nil
	case "atleastasfresh", "at_least_as_fresh", "at-least-as-fresh":
		return AtLeastAsFresh, nil
	default:
		return MinimizeLatency, fmt.Errorf("unknown consistency mode: %s", s)
	}
}
