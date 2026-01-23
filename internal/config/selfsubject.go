package config

import (
	"fmt"
	"strings"

	"github.com/project-kessel/inventory-api/internal/subject/selfsubject"
)

// SelfSubjectStrategyConfig holds settings for self-subject derivation strategies.
type SelfSubjectStrategyConfig struct {
	RedHatRbacSelfSubjectStrategy RedHatRbacSelfSubjectStrategyConfig `mapstructure:"redhatRbacSelfSubjectStratergy"`
}

// RedHatRbacSelfSubjectStrategyConfig configures the Red Hat RBAC strategy.
type RedHatRbacSelfSubjectStrategyConfig struct {
	Enabled             bool                   `mapstructure:"enabled"`
	XRhIdentityDomain   string                 `mapstructure:"issuerDomain"`
	OIDCIssuerDomainMap map[string]interface{} `mapstructure:"oidcIssuerDomainMap"`
}

func NewSelfSubjectStrategyConfig() *SelfSubjectStrategyConfig {
	return &SelfSubjectStrategyConfig{}
}

// BuildSelfSubjectStrategy constructs the configured strategy or returns nil when disabled.
func BuildSelfSubjectStrategy(cfg *SelfSubjectStrategyConfig) selfsubject.SelfSubjectStrategy {
	if cfg == nil || !cfg.RedHatRbacSelfSubjectStrategy.Enabled {
		return nil
	}

	return selfsubject.NewRedHatRbacSelfSubjectStrategy(selfsubject.RedHatRbacSelfSubjectStrategyConfig{
		Enabled:             cfg.RedHatRbacSelfSubjectStrategy.Enabled,
		XRhIdentityDomain:   cfg.RedHatRbacSelfSubjectStrategy.XRhIdentityDomain,
		OIDCIssuerDomainMap: normalizeIssuerDomainMap(cfg.RedHatRbacSelfSubjectStrategy.OIDCIssuerDomainMap),
	})
}

func normalizeIssuerDomainMap(input map[string]interface{}) map[string]string {
	flat := make(map[string]string)
	for key, value := range input {
		flattenIssuerDomainMap(flat, key, value)
	}

	if len(flat) == 0 {
		return flat
	}

	normalized := make(map[string]string)
	for key, value := range flat {
		normalized[key] = value
		normalizedKey := strings.ReplaceAll(key, ".", "_")
		normalized[normalizedKey] = value
		normalizedKey = strings.ReplaceAll(key, ".", "-")
		normalized[normalizedKey] = value
	}

	return normalized
}

func flattenIssuerDomainMap(out map[string]string, prefix string, value interface{}) {
	switch v := value.(type) {
	case string:
		out[prefix] = v
	case map[string]interface{}:
		for key, nested := range v {
			flattenIssuerDomainMap(out, fmt.Sprintf("%s.%s", prefix, key), nested)
		}
	case map[interface{}]interface{}:
		for key, nested := range v {
			keyStr, ok := key.(string)
			if !ok {
				continue
			}
			flattenIssuerDomainMap(out, fmt.Sprintf("%s.%s", prefix, keyStr), nested)
		}
	}
}
