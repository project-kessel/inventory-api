package selfsubject

import (
	"fmt"
	"strings"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/project-kessel/inventory-api/internal/biz/model"
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

// SubjectFromAuthorizationContext derives a SubjectReference for self-authorization.
func (s *RedHatRbacSelfSubjectStrategy) SubjectFromAuthorizationContext(authzContext authnapi.AuthzContext) (model.SubjectReference, error) {
	subjectID, err := s.deriveSubjectID(authzContext)
	if err != nil {
		return model.SubjectReference{}, err
	}
	if subjectID == "" {
		return model.SubjectReference{}, fmt.Errorf("subject not found")
	}

	return buildSubjectReference(subjectID)
}

// deriveSubjectID derives the domain-qualified subject ID string from the authorization context.
func (s *RedHatRbacSelfSubjectStrategy) deriveSubjectID(authzContext authnapi.AuthzContext) (string, error) {
	if s == nil || !s.enabled {
		return "", fmt.Errorf("self-subject strategy disabled")
	}
	// Un-authenticated requests - we cannot derive a subject for them
	if !authzContext.IsAuthenticated() {
		return "", nil
	}

	claims := authzContext.Subject
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

func (s *RedHatRbacSelfSubjectStrategy) resolveOIDCIssuerDomain(issuer string) string {
	if issuer == "" || s.oidcIssuerDomains == nil {
		return ""
	}
	if domain, ok := s.oidcIssuerDomains[issuer]; ok && domain != "" {
		return domain
	}
	return ""
}

func extractSubjectID(subject string) string {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return ""
	}
	return subject
}

// buildSubjectReference creates a SubjectReference for RBAC authorization.
// Uses fixed values: namespace="rbac", resource type="principal".
func buildSubjectReference(subjectID string) (model.SubjectReference, error) {
	localResourceId, err := model.NewLocalResourceId(subjectID)
	if err != nil {
		return model.SubjectReference{}, fmt.Errorf("invalid subject ID: %w", err)
	}
	resourceType, err := model.NewResourceType("principal")
	if err != nil {
		return model.SubjectReference{}, fmt.Errorf("invalid resource type: %w", err)
	}
	reporterType, err := model.NewReporterType("rbac")
	if err != nil {
		return model.SubjectReference{}, fmt.Errorf("invalid reporter type: %w", err)
	}

	key, err := model.NewReporterResourceKey(localResourceId, resourceType, reporterType, model.ReporterInstanceId(""))
	if err != nil {
		return model.SubjectReference{}, fmt.Errorf("failed to build subject key: %w", err)
	}

	return model.NewSubjectReferenceWithoutRelation(key), nil
}
