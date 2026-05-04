package resources

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSelfSubjectOptions_ReturnsDefaults(t *testing.T) {
	opts := NewSelfSubjectOptions()

	assert.NotNil(t, opts)
	assert.NotNil(t, opts.RedHatRbac)
	assert.False(t, opts.RedHatRbac.Enabled)
}

func TestSelfSubjectOptions_Validate_DisabledPasses(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = false

	errs := opts.Validate()
	assert.Empty(t, errs)
}

func TestSelfSubjectOptions_Validate_EnabledRequiresDomain(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = ""

	errs := opts.Validate()
	assert.Len(t, errs, 2)
	assert.Contains(t, errs[0].Error(), "issuer-domain is required")
	assert.Contains(t, errs[1].Error(), "oidcIssuerDomains is required")
}

func TestSelfSubjectOptions_Validate_EnabledRequiresOIDCIssuerDomains(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"
	opts.RedHatRbac.OIDCIssuerDomainMap = nil
	opts.RedHatRbac.OIDCIssuerDomains = nil

	errs := opts.Validate()
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Error(), "oidcIssuerDomains is required")
}

func TestSelfSubjectOptions_Validate_EnabledWithDomainPasses(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"
	opts.RedHatRbac.OIDCIssuerDomainMap = map[string]string{
		"https://sso.redhat.com/auth/realms/redhat-external": "redhat",
	}

	errs := opts.Validate()
	assert.Empty(t, errs)
}

func TestSelfSubjectOptions_Build_DisabledReturnsNil(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = false

	strategy := opts.Build()
	assert.Nil(t, strategy)
}

func TestSelfSubjectOptions_Build_EnabledReturnsStrategy(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"

	strategy := opts.Build()
	assert.NotNil(t, strategy)
}

func TestSelfSubjectOptions_Build_PassesIssuerDomainMap(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"
	opts.RedHatRbac.OIDCIssuerDomainMap = map[string]string{
		"https://sso.redhat.com/auth/realms/redhat-external": "redhat",
		"https://login.example.com":                          "example",
	}

	errs := opts.Complete()
	assert.Empty(t, errs)

	strategy := opts.Build()
	assert.NotNil(t, strategy)

	rbacStrategy := strategy.(*RedHatRbacSelfSubjectStrategy)
	assert.Equal(t, "redhat", rbacStrategy.oidcIssuerDomains["https://sso.redhat.com/auth/realms/redhat-external"])
	assert.Equal(t, "example", rbacStrategy.oidcIssuerDomains["https://login.example.com"])
}

func TestSelfSubjectOptions_Complete_UsesIssuerDomainsList(t *testing.T) {
	opts := NewSelfSubjectOptions()
	opts.RedHatRbac.Enabled = true
	opts.RedHatRbac.XRhIdentityDomain = "redhat"
	opts.RedHatRbac.OIDCIssuerDomains = []OIDCIssuerDomainEntry{
		{
			Issuer: "https://sso.redhat.com/auth/realms/redhat-external",
			Domain: "redhat",
		},
		{
			Issuer: "https://login.example.com",
			Domain: "example",
		},
	}

	errs := opts.Complete()
	assert.Empty(t, errs)

	strategy := opts.Build()
	assert.NotNil(t, strategy)

	rbacStrategy := strategy.(*RedHatRbacSelfSubjectStrategy)
	assert.Equal(t, "redhat", rbacStrategy.oidcIssuerDomains["https://sso.redhat.com/auth/realms/redhat-external"])
	assert.Equal(t, "example", rbacStrategy.oidcIssuerDomains["https://login.example.com"])
}
