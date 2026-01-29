package selfsubject

import (
	"testing"

	authnapi "github.com/project-kessel/inventory-api/internal/authn/api"
	"github.com/stretchr/testify/assert"
)

func TestRedHatRbacSelfSubjectStrategy_XRhIdentity_User(t *testing.T) {
	strategy := NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled:           true,
		XRhIdentityDomain: "redhat",
	})

	subjectRef, err := strategy.SubjectFromAuthorizationContext(authnapi.AuthzContext{
		Claims: &authnapi.Claims{
			AuthType:  authnapi.AuthTypeXRhIdentity,
			SubjectId: authnapi.SubjectId("user-123"),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "redhat/user-123", subjectRef.Subject().LocalResourceId().String())
	assert.Equal(t, "principal", subjectRef.Subject().ResourceType().String())
	assert.Equal(t, "rbac", subjectRef.Subject().ReporterType().String())
}

func TestRedHatRbacSelfSubjectStrategy_OIDC_IssuerMap(t *testing.T) {
	strategy := NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled: true,
		OIDCIssuerDomainMap: map[string]string{
			"https://sso.redhat.com/auth/realms/redhat-external": "redhat",
		},
	})

	subjectRef, err := strategy.SubjectFromAuthorizationContext(authnapi.AuthzContext{
		Claims: &authnapi.Claims{
			AuthType:  authnapi.AuthTypeOIDC,
			Issuer:    authnapi.Issuer("https://sso.redhat.com/auth/realms/redhat-external"),
			SubjectId: authnapi.SubjectId("user-123"),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "redhat/user-123", subjectRef.Subject().LocalResourceId().String())
}

func TestRedHatRbacSelfSubjectStrategy_OIDC_IssuerHostFallback(t *testing.T) {
	strategy := NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled: true,
		OIDCIssuerDomainMap: map[string]string{
			"sso.redhat.com": "redhat",
		},
	})

	subjectRef, err := strategy.SubjectFromAuthorizationContext(authnapi.AuthzContext{
		Claims: &authnapi.Claims{
			AuthType:  authnapi.AuthTypeOIDC,
			Issuer:    authnapi.Issuer("https://sso.redhat.com/auth/realms/redhat-external"),
			SubjectId: authnapi.SubjectId("user-123"),
		},
	})
	assert.NoError(t, err)
	assert.Equal(t, "redhat/user-123", subjectRef.Subject().LocalResourceId().String())
}

func TestRedHatRbacSelfSubjectStrategy_OIDC_NormalizedHostNotSupported(t *testing.T) {
	strategy := NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled: true,
		OIDCIssuerDomainMap: map[string]string{
			"sso-redhat-com": "redhat",
			"sso_redhat_com": "redhat",
		},
	})

	_, err := strategy.SubjectFromAuthorizationContext(authnapi.AuthzContext{
		Claims: &authnapi.Claims{
			AuthType:  authnapi.AuthTypeOIDC,
			Issuer:    authnapi.Issuer("https://sso.redhat.com/auth/realms/redhat-external"),
			SubjectId: authnapi.SubjectId("user-123"),
		},
	})
	assert.Error(t, err)
}

func TestRedHatRbacSelfSubjectStrategy_OIDC_MissingIssuer(t *testing.T) {
	strategy := NewRedHatRbacSelfSubjectStrategy(RedHatRbacSelfSubjectStrategyConfig{
		Enabled: true,
	})

	_, err := strategy.SubjectFromAuthorizationContext(authnapi.AuthzContext{
		Claims: &authnapi.Claims{
			AuthType:  authnapi.AuthTypeOIDC,
			SubjectId: authnapi.SubjectId("user-123"),
		},
	})
	assert.Error(t, err)
}
