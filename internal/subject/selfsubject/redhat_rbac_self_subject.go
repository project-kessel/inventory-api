package selfsubject

import (
	"fmt"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
)

// RedHatRbacSelfSubjectStrategy implements RBAC subject derivation for Red Hat identity domains.
type RedHatRbacSelfSubjectStrategy struct {
	enabled           bool
	xRhIdentityDomain string
	oidcIssuerDomains map[string]string
}

// NewRedHatRbacSelfSubjectStrategy constructs the strategy from config.
func NewRedHatRbacSelfSubjectStrategy(cfg RedHatRbacSelfSubjectStrategyConfig) *RedHatRbacSelfSubjectStrategy {
	return &RedHatRbacSelfSubjectStrategy{
		enabled:           cfg.Enabled,
		xRhIdentityDomain: cfg.XRhIdentityDomain,
		oidcIssuerDomains: cfg.OIDCIssuerDomainMap,
	}
}

// Enabled reports whether this strategy is configured and enabled.
func (s *RedHatRbacSelfSubjectStrategy) Enabled() bool {
	return s != nil && s.enabled
}

// SubjectFromAuthorizationContext derives a domain-qualified subject string for self-authorization.
func (s *RedHatRbacSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (string, error) {
	if s == nil || !s.enabled {
		return "", fmt.Errorf("self-subject strategy disabled")
	}
	// Un-authenticated requests  - we cannot derive a subject for them
	if !authzContext.IsAuthenticated() {
		return "", nil
	}

	claims := authzContext.Claims
	subjectID := extractSubjectID(string(claims.SubjectId))

	switch claims.AuthType {
	case authnapi.AuthTypeXRhIdentity:
		if subjectID == "" {
			return "", fmt.Errorf("missing subject for x-rh-identity")
		}
		if s.xRhIdentityDomain == "" {
			return "", fmt.Errorf("missing x-rh-identity domain configuration")
		}
		return fmt.Sprintf("%s/%s", s.xRhIdentityDomain, subjectID), nil
	case authnapi.AuthTypeOIDC:
		if claims.Issuer == "" {
			return "", fmt.Errorf("missing issuer for oidc")
		}
		domain := s.resolveOIDCIssuerDomain(string(claims.Issuer))
		if domain == "" {
			return "", fmt.Errorf("unsupported issuer for oidc")
		}
		if subjectID == "" {
			return "", fmt.Errorf("missing subject for oidc")
		}
		return fmt.Sprintf("%s/%s", domain, subjectID), nil
	default:
		return "", fmt.Errorf("unsupported auth type")
	}
}
